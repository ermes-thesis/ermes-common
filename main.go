package main

import (
	"fmt"
	"github.com/ermes-thesis/ermes-wasm-test/wasm-pool"
	"github.com/redis/go-redis/v9"
	"strconv"
	"time"
)

var client = redis.NewClient(&redis.Options{
	Addr:     "redis:6379",                       // use default Addr
	Password: "eYVX7EwVmmxKPCDmwMtyKVge8oLd2t81", // no password set
	DB:       0,                                  // use default DB
})

func main() {
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
