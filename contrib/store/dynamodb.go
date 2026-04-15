package store

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"go_agent_framework/core"
)

// DynamoStore implements core.StateStore backed by a DynamoDB table.
//
// Table schema:
//
//	Partition key: session_id (S)
//
// Attributes: state (S, JSON), version (N), updated_at (S, RFC3339).
// Put uses a version condition expression for optimistic locking.
type DynamoStore struct {
	client    *dynamodb.Client
	tableName string
}

// NewDynamoStore creates a store bound to the given DynamoDB table.
func NewDynamoStore(client *dynamodb.Client, tableName string) *DynamoStore {
	return &DynamoStore{client: client, tableName: tableName}
}

func (d *DynamoStore) Get(id string) (*core.Session, error) {
	out, err := d.client.GetItem(context.TODO(), &dynamodb.GetItemInput{
		TableName: &d.tableName,
		Key: map[string]ddbtypes.AttributeValue{
			"session_id": &ddbtypes.AttributeValueMemberS{Value: id},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("dynamodb get: %w", err)
	}

	if out.Item == nil {
		return &core.Session{
			ID:    id,
			State: make(map[string]interface{}),
		}, nil
	}

	session := &core.Session{ID: id}

	if v, ok := out.Item["state"].(*ddbtypes.AttributeValueMemberS); ok {
		if err := json.Unmarshal([]byte(v.Value), &session.State); err != nil {
			return nil, fmt.Errorf("unmarshal state: %w", err)
		}
	}
	if v, ok := out.Item["version"].(*ddbtypes.AttributeValueMemberN); ok {
		session.Version, _ = strconv.Atoi(v.Value)
	}
	if v, ok := out.Item["updated_at"].(*ddbtypes.AttributeValueMemberS); ok {
		session.UpdatedAt, _ = time.Parse(time.RFC3339, v.Value)
	}

	return session, nil
}

func (d *DynamoStore) Put(session *core.Session) error {
	stateJSON, err := json.Marshal(session.State)
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}

	newVersion := session.Version + 1
	now := time.Now()

	input := &dynamodb.PutItemInput{
		TableName: &d.tableName,
		Item: map[string]ddbtypes.AttributeValue{
			"session_id": &ddbtypes.AttributeValueMemberS{Value: session.ID},
			"state":      &ddbtypes.AttributeValueMemberS{Value: string(stateJSON)},
			"version":    &ddbtypes.AttributeValueMemberN{Value: strconv.Itoa(newVersion)},
			"updated_at": &ddbtypes.AttributeValueMemberS{Value: now.Format(time.RFC3339)},
		},
	}

	// Optimistic locking: only succeed if the stored version matches what we read,
	// or if the item does not exist yet (new session).
	if session.Version > 0 {
		input.ConditionExpression = aws.String("attribute_not_exists(session_id) OR version = :v")
		input.ExpressionAttributeValues = map[string]ddbtypes.AttributeValue{
			":v": &ddbtypes.AttributeValueMemberN{Value: strconv.Itoa(session.Version)},
		}
	}

	if _, err := d.client.PutItem(context.TODO(), input); err != nil {
		return fmt.Errorf("dynamodb put: %w", err)
	}

	session.Version = newVersion
	session.UpdatedAt = now
	return nil
}
