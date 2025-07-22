package test

import (
	"context"
	"fmt"
	"github.com/ermes-thesis/ermes-wasm-pool/wasm-pool"
	"github.com/redis/go-redis/v9"
	"github.com/testcontainers/testcontainers-go"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
	"os"
	"strings"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	os.Setenv("TESTCONTAINERS_HOST_OVERRIDE", wasm.GetDockerHost())
	os.Setenv("ERMES_SECRET", "secret")
	m.Run()
	os.Unsetenv("TESTCONTAINERS_HOST_OVERRIDE")
	os.Unsetenv("ERMES_SECRET")
}

func TestWasmPool(t *testing.T) {
	redisContainer, err := tcredis.Run(context.Background(),
		"redis:latest",
		tcredis.WithSnapshotting(10, 1),
		tcredis.WithLogLevel(tcredis.LogLevelVerbose),
		//tcredis.WithConfigFile(filepath.Join("testdata", "redis7.conf")),
	)
	defer func() {
		if err := testcontainers.TerminateContainer(redisContainer); err != nil {
			t.Fatalf("failed to terminate container: %s", err)
		}
	}()
	if err != nil {
		t.Fatalf("failed to start container: %s", err)
		return
	}

	connectionString, _ := redisContainer.ConnectionString(context.Background())
	client := redis.NewClient(&redis.Options{
		Addr: strings.Split(connectionString, "//")[1],
	})

	client.SAdd(context.Background(), "data:255ef4af-3a49-4382-a90d-6524bdee5ad1:groups", "test")
	client.Set(context.Background(), "data:255ef4af-3a49-4382-a90d-6524bdee5ad1:battiti", "100", 0)

	pool := wasm.NewExecutorPool(10)
	defer pool.Close()

	pool.RegisterHostFns(client)

	executor := pool.Acquire()
	defer executor.Release()

	userdId, code := executor.InvokeAuthFn("auth_function", "token")
	fmt.Println("Auth function:", userdId, code)

	req, ctx := getTestReqAndCtx(userdId)
	response, resCode := executor.InvokeFn("function", req, ctx)
	fmt.Println("Invoke function:", string(response), resCode)

	if resCode != 200 {
		t.Fatalf("invoke function failed: %s", string(response))
	}
}

func BenchmarkWasmPool(b *testing.B) {
	redisContainer, err := tcredis.Run(context.Background(),
		"redis:latest",
		tcredis.WithSnapshotting(10, 1),
		tcredis.WithLogLevel(tcredis.LogLevelVerbose),
		//tcredis.WithConfigFile(filepath.Join("testdata", "redis7.conf")),
	)
	defer func() {
		if err := testcontainers.TerminateContainer(redisContainer); err != nil {
			b.Fatalf("failed to terminate container: %s", err)
		}
	}()
	if err != nil {
		b.Fatalf("failed to start container: %s", err)
		return
	}

	connectionString, _ := redisContainer.ConnectionString(context.Background())
	client := redis.NewClient(&redis.Options{
		Addr: strings.Split(connectionString, "//")[1],
	})

	client.Set(context.Background(), "data:255ef4af-3a49-4382-a90d-6524bdee5ad1:battiti", "100", 0)

	pool := wasm.NewExecutorPool(10)
	defer pool.Close()

	pool.RegisterHostFns(client)

	for b.Loop() {
		client.SAdd(context.Background(), "data:255ef4af-3a49-4382-a90d-6524bdee5ad1:groups", "test")

		executor := pool.Acquire()

		userdId, code := executor.InvokeAuthFn("auth_function", "token")
		fmt.Println("Auth function:", userdId, code)

		req, ctx := getTestReqAndCtx(userdId)
		response, resCode := executor.InvokeFn("function", req, ctx)
		fmt.Println("Invoke function:", string(response), resCode)

		if resCode != 200 {
			b.Fatalf("invoke function failed: %s", string(response))
		}
		executor.Release()
	}
}

func getTestReqAndCtx(userId string) (string, string) {
	req := "{\"prova\": \"ciao\"}"

	timestamp, _ := time.Now().MarshalJSON()
	ctx := "{" +
		"\"userId\": \"" + userId + "\"," +
		"\"groupList\": [\"test\"]," +
		"\"collections\": {" +
		"\"test_collection\": [" +
		"{" +
		"\"CollectionId\": \"255ef4af-3a49-4382-a90d-6524bdee5ad1\"," +
		"\"Metadata\": {" +
		"\"consistency\": 1," +
		"\"name\": \"dati\"," +
		"\"owner\": \"pippo\"," +
		"\"createdAt\": " + string(timestamp) + "" +
		"}" +
		"}" +
		"]" +
		"}" +
		"}"
	return req, ctx
}
