package plugins

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResponseCtx(t *testing.T) {
	ctx := ResponseCtx(context.Background())
	val := ctx.Value(responseMetadataKey)
	assert.NotNil(t, val)
	assert.IsType(t, map[string]interface{}{}, val)

	assert.NotNil(t, ResponseMetadata(ctx))

	val = ctx.Value(queryMetadataKey)
	assert.Nil(t, val)

	assert.Nil(t, QueryMetadata(ctx))
}

func TestQueryCtx(t *testing.T) {
	ctx := QueryCtx(context.Background())
	val := ctx.Value(queryMetadataKey)
	assert.NotNil(t, val)
	assert.IsType(t, map[string]interface{}{}, val)

	assert.NotNil(t, QueryMetadata(ctx))

	val = ctx.Value(responseMetadataKey)
	assert.Nil(t, val)

	assert.Nil(t, ResponseMetadata(ctx))
}

func TestCreateNewHandlerCtx(t *testing.T) {
	ctx := CreateNewHandlerCtx()

	val := ResponseMetadata(ctx)
	assert.NotNil(t, val)
	assert.IsType(t, map[string]interface{}{}, val)

	val = QueryMetadata(ctx)
	assert.NotNil(t, val)
	assert.IsType(t, map[string]interface{}{}, val)
}
