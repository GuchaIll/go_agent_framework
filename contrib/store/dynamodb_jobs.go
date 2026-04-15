package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"go_agent_framework/contrib/queue"
)

// JobStore tracks async job status in a DynamoDB table.
//
// Table schema:
//
//	Partition key: job_id (S)
//	GSI: session_id-index (partition key: session_id)
//
// Attributes: graph_name, session_id, status, input, result, error,
// created_at, updated_at, ttl (N, epoch seconds for DynamoDB TTL).
type JobStore struct {
	client    *dynamodb.Client
	tableName string
	ttlDays   int // auto-expire completed jobs after this many days (0 = no TTL)
}

// NewJobStore creates a job store bound to the given DynamoDB table.
func NewJobStore(client *dynamodb.Client, tableName string, ttlDays int) *JobStore {
	return &JobStore{client: client, tableName: tableName, ttlDays: ttlDays}
}

// CreateJob writes a new job record with PENDING status.
func (s *JobStore) CreateJob(ctx context.Context, job *queue.Job) error {
	if job.CreatedAt.IsZero() {
		job.CreatedAt = time.Now()
	}
	job.Status = queue.JobPending
	job.UpdatedAt = job.CreatedAt

	item := map[string]ddbtypes.AttributeValue{
		"job_id":     &ddbtypes.AttributeValueMemberS{Value: job.ID},
		"graph_name": &ddbtypes.AttributeValueMemberS{Value: job.GraphName},
		"session_id": &ddbtypes.AttributeValueMemberS{Value: job.SessionID},
		"status":     &ddbtypes.AttributeValueMemberS{Value: string(job.Status)},
		"created_at": &ddbtypes.AttributeValueMemberS{Value: job.CreatedAt.Format(time.RFC3339)},
		"updated_at": &ddbtypes.AttributeValueMemberS{Value: job.UpdatedAt.Format(time.RFC3339)},
	}

	if job.Input != nil {
		inputJSON, err := json.Marshal(job.Input)
		if err != nil {
			return fmt.Errorf("marshal input: %w", err)
		}
		item["input"] = &ddbtypes.AttributeValueMemberS{Value: string(inputJSON)}
	}

	if s.ttlDays > 0 {
		ttl := job.CreatedAt.Add(time.Duration(s.ttlDays) * 24 * time.Hour).Unix()
		item["ttl"] = &ddbtypes.AttributeValueMemberN{Value: fmt.Sprintf("%d", ttl)}
	}

	_, err := s.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: &s.tableName,
		Item:      item,
	})
	if err != nil {
		return fmt.Errorf("dynamodb create job: %w", err)
	}
	return nil
}

// GetJob retrieves a job by ID.
func (s *JobStore) GetJob(ctx context.Context, jobID string) (*queue.Job, error) {
	out, err := s.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: &s.tableName,
		Key: map[string]ddbtypes.AttributeValue{
			"job_id": &ddbtypes.AttributeValueMemberS{Value: jobID},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("dynamodb get job: %w", err)
	}
	if out.Item == nil {
		return nil, fmt.Errorf("job %s not found", jobID)
	}

	return itemToJob(out.Item)
}

// UpdateStatus sets the job status and updated_at timestamp.
func (s *JobStore) UpdateStatus(ctx context.Context, jobID string, status queue.JobStatus) error {
	_, err := s.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: &s.tableName,
		Key: map[string]ddbtypes.AttributeValue{
			"job_id": &ddbtypes.AttributeValueMemberS{Value: jobID},
		},
		UpdateExpression: aws.String("SET #s = :status, updated_at = :now"),
		ExpressionAttributeNames: map[string]string{
			"#s": "status",
		},
		ExpressionAttributeValues: map[string]ddbtypes.AttributeValue{
			":status": &ddbtypes.AttributeValueMemberS{Value: string(status)},
			":now":    &ddbtypes.AttributeValueMemberS{Value: time.Now().Format(time.RFC3339)},
		},
	})
	if err != nil {
		return fmt.Errorf("dynamodb update status: %w", err)
	}
	return nil
}

// SetResult writes the job result (or error) and marks it completed or failed.
func (s *JobStore) SetResult(ctx context.Context, jobID string, result map[string]interface{}, jobErr string) error {
	status := queue.JobCompleted
	if jobErr != "" {
		status = queue.JobFailed
	}

	updateExpr := "SET #s = :status, updated_at = :now"
	exprValues := map[string]ddbtypes.AttributeValue{
		":status": &ddbtypes.AttributeValueMemberS{Value: string(status)},
		":now":    &ddbtypes.AttributeValueMemberS{Value: time.Now().Format(time.RFC3339)},
	}

	if result != nil {
		resultJSON, err := json.Marshal(result)
		if err != nil {
			return fmt.Errorf("marshal result: %w", err)
		}
		updateExpr += ", #r = :result"
		exprValues[":result"] = &ddbtypes.AttributeValueMemberS{Value: string(resultJSON)}
	}

	if jobErr != "" {
		updateExpr += ", #e = :error"
		exprValues[":error"] = &ddbtypes.AttributeValueMemberS{Value: jobErr}
	}

	exprNames := map[string]string{"#s": "status"}
	if result != nil {
		exprNames["#r"] = "result"
	}
	if jobErr != "" {
		exprNames["#e"] = "error"
	}

	_, err := s.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: &s.tableName,
		Key: map[string]ddbtypes.AttributeValue{
			"job_id": &ddbtypes.AttributeValueMemberS{Value: jobID},
		},
		UpdateExpression:          aws.String(updateExpr),
		ExpressionAttributeNames:  exprNames,
		ExpressionAttributeValues: exprValues,
	})
	if err != nil {
		return fmt.Errorf("dynamodb set result: %w", err)
	}
	return nil
}

func itemToJob(item map[string]ddbtypes.AttributeValue) (*queue.Job, error) {
	job := &queue.Job{}

	if v, ok := item["job_id"].(*ddbtypes.AttributeValueMemberS); ok {
		job.ID = v.Value
	}
	if v, ok := item["graph_name"].(*ddbtypes.AttributeValueMemberS); ok {
		job.GraphName = v.Value
	}
	if v, ok := item["session_id"].(*ddbtypes.AttributeValueMemberS); ok {
		job.SessionID = v.Value
	}
	if v, ok := item["status"].(*ddbtypes.AttributeValueMemberS); ok {
		job.Status = queue.JobStatus(v.Value)
	}
	if v, ok := item["input"].(*ddbtypes.AttributeValueMemberS); ok {
		_ = json.Unmarshal([]byte(v.Value), &job.Input)
	}
	if v, ok := item["result"].(*ddbtypes.AttributeValueMemberS); ok {
		_ = json.Unmarshal([]byte(v.Value), &job.Result)
	}
	if v, ok := item["error"].(*ddbtypes.AttributeValueMemberS); ok {
		job.Error = v.Value
	}
	if v, ok := item["created_at"].(*ddbtypes.AttributeValueMemberS); ok {
		job.CreatedAt, _ = time.Parse(time.RFC3339, v.Value)
	}
	if v, ok := item["updated_at"].(*ddbtypes.AttributeValueMemberS); ok {
		job.UpdatedAt, _ = time.Parse(time.RFC3339, v.Value)
	}

	return job, nil
}
