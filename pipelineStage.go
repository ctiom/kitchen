package kitchen

type PipelineStage[D IPipelineCookware[M], M IPipelineModel] struct {
	SetBase[D]
	status    PipelineStatus
	_pipeline iPipeline[D, M]
	actions   []IPipelineAction
}

func initPipelineStage[D IPipelineCookware[M], M IPipelineModel](pipeline iPipeline[D, M], stage iPipelineStage[D, M], name string) {
	stage.initStage(pipeline, stage, PipelineStatus(name))
}

func (ps PipelineStage[D, M]) Status() PipelineStatus {
	return ps.status
}

func (ps *PipelineStage[D, M]) initStage(parent iCookbook[D], stage iPipelineStage[D, M], stageName PipelineStatus) {
	ps.status = stageName
	var ok bool
	if ps._menu, ok = parent.(iMenu[D]); ok {
		ps._pipeline = parent.(iPipeline[D, M])
	} else {
		ps._pipeline = parent.(iPipelineStage[D, M]).pipeline()
		ps.parentSet = parent.(iPipelineStage[D, M]).tree()
	}
	ps.self = stage
	ps.name = string(stageName)
	ps.nodes = iteratePipelineStruct(stage, ps._pipeline, nil, stage, ps._pipeline.cookware())
}

func (ps PipelineStage[D, M]) pipeline() iPipeline[D, M] {
	return ps._pipeline
}

func (ps PipelineStage[D, M]) Actions() []IPipelineAction {
	return ps.actions
}
