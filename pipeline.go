package kitchen

import (
	"reflect"
)

type PipelineBase[P iPipeline[D, M], D IPipelineCookware[M], M IPipelineModel] struct {
	MenuBase[P, D]
	StageByStatus map[string]iPipelineStage[D, M]
}

func InitPipeline[D IPipelineCookware[M], M IPipelineModel, P iPipeline[D, M]](pipelinePtr P, dep D) P {
	pipelinePtr.initPipeline(pipelinePtr, dep)
	return pipelinePtr
}

func (p *PipelineBase[P, D, M]) initPipeline(pipeline iPipeline[D, M], dep D) {
	pipeline.initWithoutFields(pipeline, dep)
	p.StageByStatus = map[string]iPipelineStage[D, M]{}
	p.nodes = iteratePipelineStruct[D, M](pipeline, pipeline, p.StageByStatus, nil, dep)
}

func iteratePipelineStruct[D IPipelineCookware[M], M IPipelineModel](s any, pipeline iPipeline[D, M], stageByStatus map[string]iPipelineStage[D, M], stage iPipelineStage[D, M], bundle D) []iCookbook[D] {
	var (
		ppValue   = reflect.ValueOf(s).Elem()
		ppType    = ppValue.Type()
		fieldType reflect.StructField
		nodes     []iCookbook[D]
		path      string
	)

	for i, l := 0, ppType.NumField(); i < l; i++ {
		fieldType = ppType.Field(i)
		path = fieldType.Name
		if fieldType.IsExported() && !fieldType.Anonymous {
			if fieldType.Type.Implements(typeOfPipelineAction) {
				action := ppValue.Field(i).Addr().Interface()
				initPipelineAction(pipeline.(iCookbook[D]), action.(iPipelineAction[D, M]), path, fieldType.Tag)
				nodes = append(nodes, action.(iCookbook[D]))
			} else if fieldType.Type.Implements(typeOfDish) {
				action := ppValue.Field(i).Addr().Interface()
				initDish(pipeline.(iCookbook[D]), action.(iDish[D]), path, fieldType.Tag)
				nodes = append(nodes, action.(iCookbook[D]))
			} else if fieldType.Type.Implements(typeOfISet) {
				set := ppValue.Field(i).Addr().Interface()
				if stage, ok := set.(iPipelineStage[D, M]); ok {
					initPipelineStage(pipeline, stage, path)
					nodes = append(nodes, stage.(iCookbook[D]))
					stageByStatus[string(stage.Status())] = stage
				} else {
					initSet(pipeline.(iMenu[D]), set.(iSet[D]), stage.(iSet[D]), path)
					nodes = append(nodes, set.(iCookbook[D]))
				}
			} else if fieldType.Type.Kind() == reflect.Struct {
				nodes = append(nodes, iteratePipelineStruct[D, M](ppValue.Field(i).Addr().Interface(), pipeline, stageByStatus, stage, bundle)...)
			}
		}
	}
	return nodes
}

func (p *PipelineBase[P, D, M]) GetActionsForStatus(status string) []IPipelineAction {
	if stage, ok := p.StageByStatus[status]; ok {
		return stage.Actions()
	}
	return nil
}

func (p *PipelineBase[P, D, M]) GetActionsForModel(model any) (string, []IPipelineAction) {
	status := string(model.(IPipelineModel).GetStatus())
	return status, p.GetActionsForStatus(status)
}
