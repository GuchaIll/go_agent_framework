package core

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestLoggingAgentLogsSuccessAndFailure(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		logger, buf := bufferedLogger()
		agent := &LoggingAgent{
			Inner: &fakeAgent{name: "worker"},
		}

		err := agent.Run(&Context{State: map[string]interface{}{}, Logger: logger})
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		output := buf.String()
		if !strings.Contains(output, "agent started") || !strings.Contains(output, "agent finished") {
			t.Fatalf("expected start and finish logs, got %q", output)
		}
		if !strings.Contains(output, "name=worker") {
			t.Fatalf("expected agent name in logs, got %q", output)
		}
	})

	t.Run("failure", func(t *testing.T) {
		logger, buf := bufferedLogger()
		sentinel := errors.New("boom")
		agent := &LoggingAgent{
			Inner: &fakeAgent{
				name: "worker",
				run:  func(*Context) error { return sentinel },
			},
		}

		err := agent.Run(&Context{State: map[string]interface{}{}, Logger: logger})
		if !errors.Is(err, sentinel) {
			t.Fatalf("expected wrapped error %v, got %v", sentinel, err)
		}
		output := buf.String()
		if !strings.Contains(output, "agent started") || !strings.Contains(output, "agent failed") {
			t.Fatalf("expected start and failure logs, got %q", output)
		}
	})
}

func TestRetryableAgentRetriesUntilSuccess(t *testing.T) {
	attempts := 0
	sentinel := errors.New("temporary")
	agent := &RetryableAgent{
		Inner: &fakeAgent{
			name: "retry",
			run: func(*Context) error {
				attempts++
				if attempts < 3 {
					return sentinel
				}
				return nil
			},
		},
		MaxRetry: 3,
		Backoff:  0,
	}

	if err := agent.Run(newTestContext()); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
	}
}

func TestRetryableAgentReturnsWrappedErrorAfterExhaustion(t *testing.T) {
	attempts := 0
	sentinel := errors.New("still failing")
	agent := &RetryableAgent{
		Inner: &fakeAgent{
			name: "retry",
			run: func(*Context) error {
				attempts++
				return sentinel
			},
		},
		MaxRetry: 2,
		Backoff:  0,
	}

	err := agent.Run(newTestContext())
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected wrapped error %v, got %v", sentinel, err)
	}
	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
	}
	if !strings.Contains(err.Error(), "failed after 2 retries") {
		t.Fatalf("expected retry count in error, got %v", err)
	}
}

func TestConditionalAgentBranches(t *testing.T) {
	t.Run("then branch", func(t *testing.T) {
		called := ""
		agent := &ConditionalAgent{
			ConditionName: "check",
			Condition:     func(*Context) bool { return true },
			Then: &fakeAgent{
				name: "then",
				run: func(*Context) error {
					called = "then"
					return nil
				},
			},
			Else: &fakeAgent{
				name: "else",
				run: func(*Context) error {
					called = "else"
					return nil
				},
			},
		}

		if err := agent.Run(newTestContext()); err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if called != "then" {
			t.Fatalf("expected then branch, got %q", called)
		}
	})

	t.Run("else branch", func(t *testing.T) {
		called := ""
		agent := &ConditionalAgent{
			ConditionName: "check",
			Condition:     func(*Context) bool { return false },
			Then:          &fakeAgent{name: "then"},
			Else: &fakeAgent{
				name: "else",
				run: func(*Context) error {
					called = "else"
					return nil
				},
			},
		}

		if err := agent.Run(newTestContext()); err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if called != "else" {
			t.Fatalf("expected else branch, got %q", called)
		}
	})

	t.Run("nil else noops", func(t *testing.T) {
		agent := &ConditionalAgent{
			ConditionName: "check",
			Condition:     func(*Context) bool { return false },
			Then:          &fakeAgent{name: "then"},
		}

		if err := agent.Run(newTestContext()); err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
	})
}

func TestTimeoutAgentReturnsInnerResultBeforeDeadline(t *testing.T) {
	sentinel := errors.New("inner")
	tests := []struct {
		name string
		run  func(*Context) error
		want error
	}{
		{
			name: "success",
			run:  func(*Context) error { return nil },
		},
		{
			name: "error",
			run:  func(*Context) error { return sentinel },
			want: sentinel,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			agent := &TimeoutAgent{
				Inner:   &fakeAgent{name: "inner", run: tc.run},
				Timeout: 100 * time.Millisecond,
			}

			err := agent.Run(newTestContext())
			if !errors.Is(err, tc.want) {
				t.Fatalf("expected error %v, got %v", tc.want, err)
			}
		})
	}
}

func TestTimeoutAgentTimesOut(t *testing.T) {
	block := make(chan struct{})
	defer close(block)

	agent := &TimeoutAgent{
		Inner: &fakeAgent{
			name: "slow",
			run: func(*Context) error {
				<-block
				return nil
			},
		},
		Timeout: 20 * time.Millisecond,
	}

	err := agent.Run(newTestContext())
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("expected timeout message, got %v", err)
	}
}
