package kitchen

import (
	"context"
	"database/sql"
	"net/http"
	"reflect"
)

type (
	DishCooker[D ICookware, I any, O any] func(IContext[D], I) (output O, err error)

	PipelineDishCooker[D IPipelineCookware[M], M IPipelineModel, I any, O any] func(IPipelineContext[D, M], M, I) (output O, toSaveCanNil M, err error)

	AfterListenHandlers[D ICookware, I any, O any] func(ctx IContext[D], input I, output O, err error)

	IWebParsableInput interface {
		ParseRequestToInput(r *http.Request) (raw []byte, err error)
	}
	ICookware interface {
	}
	ICookwareInheritable interface {
		ICookware
		Inherit(ICookware) ICookware
	}
	IWebCookware interface {
		ICookware
		RequestParser(action IDish, w http.ResponseWriter, r *http.Request) (IWebCookware, error) //parse user, permission
	}
	WebErr struct {
		Err        error
		HttpStatus int
		Extra      map[string]string
		ErrorCode  int32
	}
	IWebCookwareWithDataWrapper interface {
		ICookware
		WrapWebOutput(output any, err error) (wrappedOutput any, httpStatus int)
	}
	ITraceableCookware interface {
		StartTrace(ctx context.Context, id string, spanName string, input any) (context.Context, ITraceSpan)
	}
	ITraceSpan interface {
		Detail() any
		End(output any, err error)
		AddEvent(name string, attrSets ...map[string]any)
		SetAttributes(key string, value any)
		Raw() any
		logSideEffect(ctx context.Context, instanceName string, toLog []any) (context.Context, ITraceSpan)
	}
	IInstance interface {
		ToSwagger(prefix []string, options ...SwaggerOption) (api map[string]any, types map[string]any)
		Name() string
		Menu() IMenu
		ifLock() func()
	}
	iCookbook[D ICookware] interface {
		IInstance
		inherit(...iCookbook[D])
		emitAfterCook(IContext[D], any, any, error)
		emitAfterExec(IContext[D], any, any, error)
		ToRouter(prefix ...string) map[string]map[string]http.HandlerFunc
		isTraceableDep() bool
		isInheritableDep() bool
	}
	IKitchen interface {
		Order(dish IDish, input any) (output interface{}, err error)
		canHandle(uint32, int32) bool
	}
	IManager interface {
		AddMenu(menuInitializer func() IMenu) IManager
		AddKitchen(IKitchen) IManager
		Order(dish IDish, input any) (output interface{}, err error)
	}
	IMenu interface {
		IInstance
		Name() string
		SwaggerOption(SwaggerOption) IMenu
		swaggerOption() SwaggerOption
		isDataWrapper() bool
		Manager() IManager
		setManager(m IManager, id uint32)
		ID() uint32
	}
	iMenu[D ICookware] interface {
		IMenu
		iCookbook[D]
		init(iMenu[D], D)
		initWithoutFields(iMenu[D], D)
		setName(name string)
		setCookware(D)
		Cookware() D
		Dependency() D
		pushDish(iDish[D]) int
		Dishes() []iDish[D]
	}
	ISet interface {
		Name() string
		Menu() IMenu
	}
	iSet[D ICookware] interface {
		ISet
		iCookbook[D]
		init(menu iMenu[D], group iSet[D], parent iSet[D], name string)
		Tree() []iSet[D]
		menu() iMenu[D]
	}
	IDish interface {
		Name() string
		FullName() string
		IO() (any, any)
		Desc() string
		OperationId() string
		Summary() string
		Tags() []string
		Id() uint32
		Menu() IMenu
		Sets() []ISet
		Security() []string
	}
	iDish[D ICookware] interface {
		IDish
		iCookbook[D]
		init(iCookbook[D], iDish[D], string, reflect.StructTag)
		ServeHttp() (method string, urlParams []string, handler http.HandlerFunc)
		urlAndMethod() (url string, urlParams, queryParams []paramType, method string, queryParamsRequired []string)
	}
	IContextWithSession interface {
		context.Context
		Session(...IDishServe) []IDishServe
		SetCtx(context.Context)
		RawCookware() ICookware
	}
	IDishServe interface {
		finish(output any, err error)
		Record() (action IDish, finish bool, input any, output any, err error)
	}
	IContext[D ICookware] interface {
		IContextWithSession
		Menu() iMenu[D]
		Sets() []iSet[D]
		Dish() iDish[D]
		Dependency() D
		Cookware() D
		traceableCookware() ITraceableCookware
		startTrace(name string, id string, input any) ITraceSpan
		logSideEffect(instanceName string, toLog []any) (IContext[D], ITraceSpan)
		FromWeb() *webBundle
		GetCtx() context.Context
		TraceSpan() ITraceSpan
		servedWeb()
	}
	IPipelineContext[D IPipelineCookware[M], M IPipelineModel] interface {
		IContext[D]
		Tx() IDbTx
		Pipeline() iPipeline[D, M]
		Stage() iPipelineStage[D, M]
	}
	PipelineStatus string
	IPipelineModel interface {
		GetStatus() PipelineStatus
		SetStatus(status PipelineStatus)
		PrimaryKey() any
	}
	IPipelineModelCanMap interface {
		ToMap() map[string]string
	}
	IDbRunner interface {
		QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
		PrepareContext(ctx context.Context, query string) (*sql.Stmt, error)
		ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
		QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	}
	IDbTx interface {
		IDbRunner
		Rollback() error
		Commit() error
	}
	IPipelineCookware[M IPipelineModel] interface {
		ICookware
		BeginTx(context.Context, ...*sql.TxOptions) (IDbTx, error)
		FinishTx(IDbTx, error) error
		GetModelById(ctx context.Context, pks ...any) (M, error)
		SaveModel(db IDbRunner, model M, oStatus PipelineStatus) error
	}
	IPipeline interface {
		IMenu
		GetActionsForModel(any) (status string, actions []IPipelineAction)
		GetActionsForStatus(status string) []IPipelineAction
	}
	iPipeline[D IPipelineCookware[M], M IPipelineModel] interface {
		IPipeline
		iMenu[D]
		initPipeline(pipeline iPipeline[D, M], bundle D)
	}
	IPipelineStage interface {
		ISet
		Status() PipelineStatus
		Actions() []IPipelineAction
	}
	iPipelineStage[D IPipelineCookware[M], M IPipelineModel] interface {
		IPipelineStage
		iSet[D]
		initStage(parent iCookbook[D], stage iPipelineStage[D, M], stageName PipelineStatus)
		pipeline() iPipeline[D, M]
	}
	IPipelineAction interface {
		IDish
		Status() PipelineStatus
		ModelToMap(model IPipelineModel) map[string]string
	}
	iPipelineAction[D IPipelineCookware[M], M IPipelineModel] interface {
		IPipelineAction
		iDish[D]
		initAction(parent iCookbook[D], act iPipelineAction[D, M], name string, tags reflect.StructTag)
	}
)

func (w WebErr) Error() string {
	return w.Err.Error()
}
