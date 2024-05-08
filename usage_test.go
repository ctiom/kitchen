package kitchen

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	"google.golang.org/grpc"

	_ "github.com/lib/pq"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
)

type Deposit struct {
	MenuBase[*Deposit, DummyWorkerCookware]
	New struct {
		SetBase[DummyWorkerCookware]
		Create Dish[DummyWorkerCookware, int, int]
	}
	Pending struct {
		SetBase[DummyWorkerCookware]
		List      Dish[DummyWorkerCookware, *Filter, []*DepositOrder]
		TestA     Dish[DummyWorkerCookware, int, int]
		TestGroup struct {
			TestB Dish[DummyWorkerCookware, int, int]
		}
		TestAsync Dish[DummyWorkerCookware, int, int]
	}
	Util struct {
		SetBase[DummyWorkerCookware]
		TestC Dish[DummyWorkerCookware, int, int]
	}
	TestWebUrl      Dish[DummyWorkerCookware, any, any] `path:"notify/{order_id}" method:"POST"`
	TestNamedMethod struct {
		SetBase[DummyWorkerCookware]
		POST Dish[DummyWorkerCookware, any, any] `urlParam:"test"`
	}
}
type DepositWithTracer struct {
	MenuBase[*DepositWithTracer, DummyWorkerCookwareWithTracer]
	New struct {
		SetBase[DummyWorkerCookwareWithTracer]
		Create Dish[DummyWorkerCookwareWithTracer, int, int]
	}
	Pending struct {
		SetBase[DummyWorkerCookwareWithTracer]
		List      Dish[DummyWorkerCookwareWithTracer, *Filter, []*DepositOrder]
		TestA     Dish[DummyWorkerCookwareWithTracer, int, int]
		TestGroup struct {
			TestB Dish[DummyWorkerCookwareWithTracer, int, int]
		}
		TestAsync Dish[DummyWorkerCookwareWithTracer, int, int]
	}
	Util struct {
		SetBase[DummyWorkerCookwareWithTracer]
		TestC Dish[DummyWorkerCookwareWithTracer, int, int]
	}
	TestWebUrl      Dish[DummyWorkerCookwareWithTracer, any, any] `path:"notify/{order_id}" method:"POST"`
	TestNamedMethod struct {
		SetBase[DummyWorkerCookwareWithTracer]
		POST Dish[DummyWorkerCookwareWithTracer, any, any] `urlParam:"test"`
	}
}

var (
	logger = zerolog.New(io.Discard) //.With().Timestamp().Logger()
)

func TestInit(t *testing.T) {

	var (
		callStack []int
	)

	DepositWorker := InitMenu[*DepositWithTracer, DummyWorkerCookwareWithTracer](&DepositWithTracer{}, DummyWorkerCookwareWithTracer{
		ITraceableCookware: NewZeroLogTraceableCookware(&logger),
	})
	DepositWorker.ConcurrentLimit(1) //only single thread
	DepositWorker.Pending.TestA.SetCooker(func(ctx IContext[DummyWorkerCookwareWithTracer], input int) (output int, err error) {
		res, err := DepositWorker.Util.TestC.Exec(ctx, input)
		return res + 1, err
	})
	DepositWorker.Util.TestC.SetCooker(func(ctx IContext[DummyWorkerCookwareWithTracer], input int) (output int, err error) {
		//ctx.Bundle().Sql.Query("select 1")
		return input + 1, nil
	})
	DepositWorker.Pending.TestAsync.SetAsyncCooker(context.Background(), 100, 1, func(ctx IContext[DummyWorkerCookwareWithTracer], input int) (output int, err error) {
		fmt.Println(123)
		return input + 999, nil
	})

	//should emit to MQ broadcast to whole cluster later
	DepositWorker.Pending.TestA.AfterCook(func(ctx IContext[DummyWorkerCookwareWithTracer], input int, output int, err error) {
		callStack = append(callStack, 1)
	},
		"tell log about this callback...", //desc for log
	)
	DepositWorker.Pending.AfterCook(func(ctx IContext[DummyWorkerCookwareWithTracer], input, output any, err error) {
		callStack = append(callStack, 2)
	})
	DepositWorker.AfterCook(func(ctx IContext[DummyWorkerCookwareWithTracer], input, output any, err error) {
		callStack = append(callStack, 3)
	})
	DepositWorker.Pending.TestGroup.TestB.SetCooker(func(ctx IContext[DummyWorkerCookwareWithTracer], input int) (output int, err error) {
		//ctx.Bundle().Sql.Query("select 1")
		return input + 1, nil
	})
	input := 1
	//if traffic growth up, we can have multiple instance and implement grpc & load balance inside Exec
	res, err := DepositWorker.Pending.TestA.Exec(context.Background(), input)
	assert.Equal(t, []int{3, 1, 2, 3}, callStack) //triggered all 3 events, and deposit again since we nested call deposit.Util.TestC
	assert.Equal(t, input+2, res)
	assert.Nil(t, err)

	res, err = DepositWorker.Pending.TestGroup.TestB.Exec(context.Background(), res)
	assert.Nil(t, err)
	assert.Equal(t, input+3, res)
	assert.Equal(t, []int{3, 1, 2, 3, 2, 3}, callStack) //triggered all again except deposit.Exist.TestA

	err = DepositWorker.Pending.TestAsync.ExecAsync(context.Background(), 1, func(i int, err error) {
		assert.Equal(t, 1000, i)
		assert.Nil(t, err)
	}) //run as async
	assert.Nil(t, err)
	res, err = DepositWorker.Pending.TestAsync.Exec(context.Background(), 1) //run as sync
	assert.Nil(t, err)
	assert.Equal(t, 1000, res)

}

func TestUrl(t *testing.T) {

	DepositWorker := InitMenu[*DepositWithTracer, DummyWorkerCookwareWithTracer](&DepositWithTracer{}, DummyWorkerCookwareWithTracer{
		ITraceableCookware: NewZeroLogTraceableCookware(&logger),
	})

	method, urls, handler := DepositWorker.TestWebUrl.ServeHttp()

	assert.Equal(t, "POST", method)
	assert.Equal(t, "notify/{order_id}", urls[0])
	fmt.Println("1", urls)
	assert.NotNil(t, handler)

	method, urls, handler = DepositWorker.TestNamedMethod.POST.ServeHttp()
	fmt.Println("2", urls)
	assert.Equal(t, "POST", method)
}

func TestOtel(t *testing.T) {

	type DummyTraceableCookware struct {
		ITraceableCookware
	}

	type TestTrace struct {
		MenuBase[*TestTrace, *DummyTraceableCookware]
		TestGroup1 struct {
			SetBase[*DummyTraceableCookware]
			Test1      Dish[*DummyTraceableCookware, int, int]
			TestGroup2 struct {
				SetBase[*DummyTraceableCookware]
				Test2 Dish[*DummyTraceableCookware, int, int]
			}
		}
	}

	traceClient := otlptracegrpc.NewClient(
		otlptracegrpc.WithInsecure(),
		otlptracegrpc.WithEndpoint("123.51.206.117:4317"),
		otlptracegrpc.WithDialOption(grpc.WithBlock()))
	traceExp, traceExpErr := otlptrace.New(context.Background(), traceClient)
	if traceExpErr != nil {
		panic(traceExpErr)
	}

	res, resErr := resource.New(context.Background(),
		resource.WithFromEnv(),
		resource.WithProcess(),
		resource.WithTelemetrySDK(),
		resource.WithHost(),
		resource.WithAttributes(
			// the service name used to display traces in backends
			semconv.ServiceNameKey.String("testTraceableWorker"),
		),
	)
	if resErr != nil {
		panic(resErr)
	}

	bsp := sdktrace.NewBatchSpanProcessor(traceExp, sdktrace.WithMaxQueueSize(6144))
	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithResource(res),
		sdktrace.WithSpanProcessor(bsp),
	)
	otel.SetTracerProvider(tracerProvider)

	TraceableWorker := InitMenu[*TestTrace, *DummyTraceableCookware](&TestTrace{}, &DummyTraceableCookware{
		ITraceableCookware: NewChainTraceableCookware(
			NewZeroLogTraceableCookware(&logger),
			NewOtelTraceableCookware(otel.GetTracerProvider().Tracer(
				"testTraceableWorker",
			))),
	})
	TraceableWorker.TestGroup1.Test1.SetCooker(func(ctx IContext[*DummyTraceableCookware], input int) (output int, err error) {
		time.Sleep(100 * time.Millisecond)
		input, err = TraceableWorker.TestGroup1.TestGroup2.Test2.Exec(ctx, input)
		time.Sleep(100 * time.Millisecond)
		return input + 1, err
	})
	TraceableWorker.TestGroup1.TestGroup2.Test2.SetCooker(func(ctx IContext[*DummyTraceableCookware], input int) (output int, err error) {
		time.Sleep(100 * time.Millisecond)
		ctx.TraceSpan().AddEvent("test", map[string]interface{}{"input": input})
		return input + 1, nil
	})
	TraceableWorker.TestGroup1.TestGroup2.Test2.AfterCook(func(ctx IContext[*DummyTraceableCookware], input, output int, err error) {

	})

	res1, err := TraceableWorker.TestGroup1.Test1.Exec(context.Background(), 1)
	assert.Nil(t, err)
	assert.Equal(t, 3, res1)

	//time.Sleep(time.Second * 5) //wait for grpc flush

}

func TestPipeline(t *testing.T) {

	type DepositPipeline struct {
		PipelineBase[*DepositPipeline, *DummyPipelineCookware, *DepositOrder]
		Create  PipelineAction[*DummyPipelineCookware, *DepositOrder, any, any]
		Pending struct {
			PipelineStage[*DummyPipelineCookware, *DepositOrder]
			SendRequest PipelineAction[*DummyPipelineCookware, *DepositOrder, any, any]
		}
		WaitForCallback struct {
			PipelineStage[*DummyPipelineCookware, *DepositOrder]
			Confirm PipelineAction[*DummyPipelineCookware, *DepositOrder, any, any]
			Error   PipelineAction[*DummyPipelineCookware, *DepositOrder, any, any]
		}
	}

	psqlconn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable", "3.76.63.72", 5432, "postgres", "K6crBSh_WUZAPaJAj5DA", "oss")
	// open database
	conn, err := sql.Open("postgres", psqlconn)
	assert.Nil(t, err)
	deposit := InitPipeline[*DummyPipelineCookware, *DepositOrder, *DepositPipeline](&DepositPipeline{}, &DummyPipelineCookware{
		DummyWorkerCookwareWithTracer: DummyWorkerCookwareWithTracer{
			ITraceableCookware: NewZeroLogTraceableCookware(&logger),
		},
		PipelineSqlTxCookware: NewPipelineSqlTxCookware[*DepositOrder](conn),
	})
	assert.NotNil(t, deposit)
	deposit.Pending.SendRequest.SetCooker(func(i IPipelineContext[*DummyPipelineCookware, *DepositOrder], order *DepositOrder, a any) (output any, toSaveCanNil *DepositOrder, err error) {
		fmt.Println("Pending.SendRequest")
		//ctx.Tx()
		return a, order, nil
	}).SetNextStage(deposit.WaitForCallback)

	model := &DepositOrder{Status: "Pending"}
	res, err := deposit.Pending.SendRequest.ExecWithModel(context.Background(), model, 1)
	assert.Nil(t, err)
	assert.Equal(t, 1, res)
	assert.Equal(t, deposit.WaitForCallback.Status(), model.Status)

	res, err = deposit.Pending.SendRequest.ExecById(context.Background(), 1, 1) //get from dep
	assert.Nil(t, err)
	assert.Equal(t, 1, res)
	assert.Equal(t, deposit.WaitForCallback.Status(), model.Status)

}

type DummyWebCookwareWithWrapper struct {
	DefaultWebWrapper
}

type WebTestWrapper struct {
	MenuBase[*WebTestWrapper, *DummyWebCookwareWithWrapper]
	Test struct {
		SetBase[*DummyWebCookwareWithWrapper]
		Struct Dish[*DummyWebCookwareWithWrapper, *DummyWebInput, DummyWebInput]
	}
}

func TestWebWrapper(t *testing.T) {
	webTest := InitMenu[*WebTestWrapper, *DummyWebCookwareWithWrapper](&WebTestWrapper{}, &DummyWebCookwareWithWrapper{})
	webTest.Test.Struct.SetCooker(func(i IContext[*DummyWebCookwareWithWrapper], input *DummyWebInput) (output DummyWebInput, err error) {
		if input.A == "4" {
			return *input, &WebErr{
				Err:        fmt.Errorf("test"),
				HttpStatus: 400,
				ErrorCode:  1,
				Extra: map[string]string{
					"foo": "bar",
				},
			}
		} else {
			return *input, nil
		}
	})
	router := mux.NewRouter()
	_, _, handleStruct := webTest.Test.Struct.ServeHttp()
	router.HandleFunc("/test", handleStruct)

	go func() {
		http.ListenAndServe(":18081", router)
	}()

	time.Sleep(time.Millisecond * 100)

	resp, err := http.Get("http://localhost:18081/test?a=4&b=5&c=6")
	assert.Nil(t, err)
	assert.Equal(t, 400, resp.StatusCode)
	data, err := io.ReadAll(resp.Body)
	assert.Nil(t, err)
	assert.Equal(t, `{"Data":{"A":"4","B":"5","C":"6"},"Error":{"Code":1,"Message":"test","Extra":{"foo":"bar"}}}`, string(data))

	resp, err = http.Get("http://localhost:18081/test?a=1&b=2&c=3")
	assert.Nil(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	data, err = io.ReadAll(resp.Body)
	assert.Nil(t, err)
	assert.Equal(t, `{"Data":{"A":"1","B":"2","C":"3"}}`, string(data))

	o, err := MakeSwagger("test", []string{"xxx.com"}, "/", "0.0.0", webTest)
	assert.Nil(t, err)
	fmt.Println(string(o))
}

func TestWeb(t *testing.T) {

	type WebTest struct {
		MenuBase[*WebTest, *DummyWebCookware]
		Test struct {
			SetBase[*DummyWebCookware]
			Plain       Dish[*DummyWebCookware, string, string]
			Struct      Dish[*DummyWebCookware, *DummyWebInput, DummyWebInput]
			UrlParams   Dish[*DummyWebCookware, *DummyUrlParamInput, DummyWebInput]
			QueryParams Dish[*DummyWebCookware, *DummyQueryParamInput, DummyWebInput]
			Login       Dish[*DummyWebCookware, any, int64]
			Manual      Dish[*DummyWebCookware, any, any]
		}
		Player struct {
			SetBase[*DummyWebCookware]
			Actions struct {
				SetBase[*DummyWebCookware]
				payment Dish[*DummyWebCookware, string, string]
			} `urlParam:"test"`
		}
	}

	webTest := InitMenu[*WebTest, *DummyWebCookware](&WebTest{}, &DummyWebCookware{
		DummyWorkerCookwareWithTracer: DummyWorkerCookwareWithTracer{
			ITraceableCookware: NewZeroLogTraceableCookware(&logger),
		},
	})
	webTest.Test.Plain.SetCooker(func(ctx IContext[*DummyWebCookware], input string) (output string, err error) {
		return input, nil
	})
	webTest.Test.Struct.SetCooker(func(i IContext[*DummyWebCookware], input *DummyWebInput) (output DummyWebInput, err error) {
		return *input, nil
	})
	webTest.Test.UrlParams.SetCooker(func(i IContext[*DummyWebCookware], input *DummyUrlParamInput) (output DummyWebInput, err error) {
		return DummyWebInput{A: input.Test1, B: input.Test2}, nil
	})
	webTest.Test.QueryParams.SetCooker(func(i IContext[*DummyWebCookware], input *DummyQueryParamInput) (output DummyWebInput, err error) {
		return DummyWebInput{A: input.Test1, B: input.Test2}, nil
	})
	webTest.Test.Login.SetCooker(func(i IContext[*DummyWebCookware], input any) (output int64, err error) {
		return i.Cookware().UserId, nil
	})
	webTest.Test.Manual.SetCooker(func(i IContext[*DummyWebCookware], input any) (output any, err error) {
		i.FromWeb().Response.Header().Set("Location", "https://google.com")
		return nil, nil
	})

	router := mux.NewRouter()

	//auto export
	//routerMap := webTest.ToRouter(context.Background(), "/")
	//for url, handler := range routerMap {
	//	router.HandleFunc(url, handler)
	//}
	_, _, handlePlain := webTest.Test.Plain.ServeHttp()
	router.HandleFunc("/test/plain/{id}", handlePlain)
	_, _, handleStruct := webTest.Test.Struct.ServeHttp()
	router.HandleFunc("/test/struct/{a}/{b}/{c}", handleStruct)
	router.HandleFunc("/test/struct", handleStruct)

	_, url, handleUrlParam := webTest.Test.UrlParams.ServeHttp()
	assert.Equal(t, []string{"url_params", "{test1}", "{test2}"}, url)
	router.HandleFunc("/test/"+strings.Join(url, "/"), handleUrlParam)
	router.HandleFunc("/test/more/path/to/test/"+strings.Join(url, "/"), handleUrlParam)
	_, url, handleQueryParam := webTest.Test.QueryParams.ServeHttp()
	assert.Equal(t, []string{"query_params"}, url)
	router.HandleFunc("/test/query_params", handleQueryParam)
	_, _, handleLogin := webTest.Test.Login.ServeHttp()
	router.HandleFunc("/test/login", handleLogin)
	_, _, handleManual := webTest.Test.Manual.ServeHttp()
	router.HandleFunc("/test/manual", handleManual)

	//export swagger
	//j, _ := MakeSwagger("backoffice", "xxx.com", "/", "0.0.0", SwaggerOption{Security: map[string]any{
	//	"Bearer": map[string]any{
	//		"type": "apiKey",
	//		"name": "Authorization",
	//		"in":   "header",
	//	},
	//}}, webTest...)
	//os.WriteFile("swagger.json", j, 0666)

	go func() {
		http.ListenAndServe(":18080", router)
	}()

	time.Sleep(time.Millisecond * 100)

	resp, err := http.Get("http://localhost:18080/test/plain/123")
	assert.Nil(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	data, err := io.ReadAll(resp.Body)
	assert.Nil(t, err)
	assert.Equal(t, "123", string(data))

	resp, err = http.Get("http://localhost:18080/test/struct/1/2/3")
	assert.Nil(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	data, err = io.ReadAll(resp.Body)
	assert.Nil(t, err)
	assert.Equal(t, `{"A":"1","B":"2","C":"3"}`, string(data))

	resp, err = http.Get("http://localhost:18080/test/struct?a=4&b=5&c=6")
	assert.Nil(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	data, err = io.ReadAll(resp.Body)
	assert.Nil(t, err)
	assert.Equal(t, `{"A":"4","B":"5","C":"6"}`, string(data))

	req, err := http.NewRequest("POST", "http://localhost:18080/test/struct", bytes.NewBuffer([]byte(`{"A":"7","B":"8","C":"9"}`)))
	assert.Nil(t, err)
	resp, err = http.DefaultClient.Do(req)
	assert.Nil(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	data, err = io.ReadAll(resp.Body)
	assert.Nil(t, err)
	assert.Equal(t, `{"A":"7","B":"8","C":"9"}`, string(data))

	req, err = http.NewRequest("GET", "http://localhost:18080/test/url_params/123/234", bytes.NewBuffer([]byte(`{"Test1":"234", "Test2":"345"}`)))
	assert.Nil(t, err)
	resp, err = http.DefaultClient.Do(req)
	assert.Nil(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	data, err = io.ReadAll(resp.Body)
	assert.Nil(t, err)
	assert.Equal(t, `{"A":"123","B":"234","C":""}`, string(data))

	req, err = http.NewRequest("POST", "http://localhost:18080/test/more/path/to/test/url_params/345/456", bytes.NewBuffer([]byte(`{"Test1":"567","Test1":"678"}`)))
	assert.Nil(t, err)
	resp, err = http.DefaultClient.Do(req)
	assert.Nil(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	data, err = io.ReadAll(resp.Body)
	assert.Nil(t, err)
	assert.Equal(t, `{"A":"345","B":"456","C":""}`, string(data))

	req, err = http.NewRequest("GET", "http://localhost:18080/test/query_params?qtest1=567&qtest2=678", bytes.NewBuffer([]byte(`{"Test1":"890", "Test2":"901"}`)))
	assert.Nil(t, err)
	resp, err = http.DefaultClient.Do(req)
	assert.Nil(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	data, err = io.ReadAll(resp.Body)
	assert.Nil(t, err)
	assert.Equal(t, `{"A":"567","B":"678","C":""}`, string(data))

	req, err = http.NewRequest("GET", "http://localhost:18080/test/login", nil)
	assert.Nil(t, err)
	req.Header.Set("UserId", "1234") //DummyWebCookware implements IWebCookware will parse this
	resp, err = http.DefaultClient.Do(req)
	assert.Nil(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	data, err = io.ReadAll(resp.Body)
	assert.Nil(t, err)
	assert.Equal(t, `1234`, string(data))

	resp, err = http.Get("http://localhost:18080/test/manual")
	assert.Nil(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, "https://google.com", resp.Header.Get("Location"))
}

type Filter struct {
	Ids   []int
	Users []int
}

type DummyWorkerCookware struct {
	Sql        *sql.DB
	Clickhouse *sql.DB
	//Redis      *redis.Client
	//Mq
	//Grpc
	//...
}

type DummyWorkerCookwareWithTracer struct {
	ITraceableCookware
	DummyWorkerCookware
}

type DummyWebCookware struct {
	DummyWorkerCookwareWithTracer
	UserId      int64
	HttpRequest *http.Request
	HttpWriter  http.ResponseWriter
}

func (d DummyWebCookware) RequestParser(action IDish, w http.ResponseWriter, r *http.Request) (IWebCookware, error) {
	dd := d
	dd.UserId, _ = strconv.ParseInt(r.Header.Get("UserId"), 10, 64)
	fmt.Println(r.Header)
	dd.HttpRequest = r
	dd.HttpWriter = w
	return &dd, nil
}

type DummyWebInput struct {
	A string
	B string
	C string
}

type DummyUrlParamInput struct {
	Test1 string `urlParam:"test1"`
	Test2 string `urlParam:"test2"`
}

type DummyQueryParamInput struct {
	Test1 string `queryParam:"qtest1"`
	Test2 string `queryParam:"qtest2"`
}

type DummyPipelineCookware struct {
	DummyWorkerCookwareWithTracer
	*PipelineSqlTxCookware[*DepositOrder]
}

func (d DepositOrder) GetStatus() PipelineStatus {
	return d.Status
}

func (d *DepositOrder) SetStatus(status PipelineStatus) {
	d.Status = status
}

func TestRoutineLimit(t *testing.T) {
	DepositWorkerWithTracer.ConcurrentLimit(10)
	DepositWorkerWithTracer.Pending.TestA.ConcurrentLimit(5)
	var run int64
	DepositWorkerWithTracer.Pending.TestA.SetCooker(func(ctx IContext[DummyWorkerCookwareWithTracer], input int) (output int, err error) {
		time.Sleep(time.Millisecond * 100)
		atomic.AddInt64(&run, 1)
		return input + 1, nil
	})
	for i := 0; i < 10; i++ {
		go DepositWorker.Pending.TestA.Exec(context.Background(), i)
	}
	time.Sleep(time.Millisecond * 150)
	assert.Equal(t, int64(5), run)
}

var (
	DepositWorker           *Deposit
	DepositWorkerWithTracer *DepositWithTracer
)

func init() {
	DepositWorker = InitMenu[*Deposit, DummyWorkerCookware](&Deposit{}, DummyWorkerCookware{})
	DepositWorkerWithTracer = InitMenu[*DepositWithTracer, DummyWorkerCookwareWithTracer](&DepositWithTracer{}, DummyWorkerCookwareWithTracer{
		ITraceableCookware: NewZeroLogTraceableCookware(&logger),
	})
	DepositWorker.Pending.TestA.SetCooker(func(ctx IContext[DummyWorkerCookware], input int) (output int, err error) {
		return input + 1, nil
	}).AfterCook(func(ctx IContext[DummyWorkerCookware], input, output int, err error) {
		input++
	})
	DepositWorkerWithTracer.Pending.TestA.SetCooker(func(ctx IContext[DummyWorkerCookwareWithTracer], input int) (output int, err error) {
		return input + 1, nil
	}).AfterCook(func(ctx IContext[DummyWorkerCookwareWithTracer], input, output int, err error) {
		input++
	})

}
func BenchmarkNewCtx(b *testing.B) {
	ctx := context.Background()
	for i := 0; i < b.N; i++ {
		DepositWorker.Pending.TestA.newCtx(ctx)
	}
}
func BenchmarkExec(b *testing.B) {
	var res int
	ctx := context.Background()
	for i := 0; i < b.N; i++ {
		if res, _ = DepositWorker.Pending.TestA.Exec(ctx, i); res != i+1 {
			b.Fail()
		}
	}
}
func BenchmarkExecWithTracer(b *testing.B) {
	var res int
	ctx := context.Background()
	for i := 0; i < b.N; i++ {
		if res, _ = DepositWorkerWithTracer.Pending.TestA.Exec(ctx, i); res != i+1 {
			b.Fail()
		}
	}
}
func BenchmarkExecWithTracerOff(b *testing.B) {
	var (
		res    int
		logger = logger.Level(zerolog.Disabled)
	)
	DepositWorkerWithTracer.Dependency().ITraceableCookware.(*ZeroLogTraceableCookware).Logger = &logger
	ctx := context.Background()
	for i := 0; i < b.N; i++ {
		if res, _ = DepositWorkerWithTracer.Pending.TestA.Exec(ctx, i); res != i+1 {
			b.Fail()
		}
	}
}
