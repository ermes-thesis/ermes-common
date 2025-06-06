package test

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ermes-thesis/ermes-wasm-pool/wasm-pool"
	"github.com/redis/go-redis/v9"
	"github.com/testcontainers/testcontainers-go"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
)

func TestWasmPool(t *testing.T) {
	redisContainer, err := tcredis.Run(context.Background(),
		"redis:latest",
		tcredis.WithSnapshotting(10, 1),
		tcredis.WithLogLevel(tcredis.LogLevelVerbose),
		//tcredis.WithConfigFile(filepath.Join("testdata", "redis7.conf")),
	)

	connectionString, _ := redisContainer.ConnectionString(context.Background())
	client := redis.NewClient(&redis.Options{
		Addr: strings.Split(connectionString, "//")[1],
	})

	client.SAdd(context.Background(), "data:255ef4af-3a49-4382-a90d-6524bdee5ad1:groups", "test")
	client.Set(context.Background(), "data:255ef4af-3a49-4382-a90d-6524bdee5ad1:battiti", "100", 0)

	defer func() {
		if err := testcontainers.TerminateContainer(redisContainer); err != nil {
			t.Fatalf("failed to terminate container: %s", err)
		}
	}()
	if err != nil {
		t.Fatalf("failed to start container: %s", err)
		return
	}

	pool := wasm.NewExecutorPool(10)
	defer pool.Close()

	pool.RegisterHostFns(client)

	executor := pool.Acquire()
	defer executor.Release()

	userdId, code := executor.InvokeAuthFn("auth_function", "token")
	fmt.Println("Auth function:", userdId, code)

	req, ctx := getTestReqAndCtx()
	response, resCode := executor.InvokeFn("function", req, ctx)
	fmt.Println("Invoke function:", string(response), resCode)

	if resCode != 200 {
		t.Fatalf("invoke function failed: %s", string(response))
	}
}

func getTestReqAndCtx() (string, string) {
	req := "{\"prova\": \"ciao\"}"

	ctx := "{" +
		"\"userId\": \"user\"," +
		"\"groupList\": [\"test\"]," +
		"\"collections\": {" +
		"\"test_collection\": [" +
		"{" +
		"\"CollectionId\": \"255ef4af-3a49-4382-a90d-6524bdee5ad1\"," +
		"\"Metadata\": {" +
		"\"consistency\": 1," +
		"\"name\": \"dati\"," +
		"\"owner\": \"pippo\"," +
		"\"created_at\": " + strconv.FormatInt(time.Now().UnixMilli(), 10) +
		"}" +
		"}" +
		"]" +
		"}" +
		"}"
	return req, ctx
}
