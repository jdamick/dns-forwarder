package plugins

import (
	"context"
)

// Context Metadata

func ResponseMetadata(ctx context.Context) map[string]interface{} {
	val := ctx.Value(responseMetadataKey)
	if val == nil {
		return nil
	}
	return val.(map[string]interface{})
}
func QueryMetadata(ctx context.Context) map[string]interface{} {
	val := ctx.Value(queryMetadataKey)
	if val == nil {
		return nil
	}
	return val.(map[string]interface{})
}

type metadataKeyType string

const (
	responseMetadataKey = metadataKeyType("responseMetadata")
	queryMetadataKey    = metadataKeyType("queryMetadata")
)

func CreateNewHandlerCtx() context.Context {
	return ResponseCtx(QueryCtx(context.Background()))
}

func ResponseCtx(ctx context.Context) context.Context {
	metadata := make(map[string]interface{})
	return context.WithValue(ctx, responseMetadataKey, metadata)
}
func QueryCtx(ctx context.Context) context.Context {
	metadata := make(map[string]interface{})
	return context.WithValue(ctx, queryMetadataKey, metadata)
}
