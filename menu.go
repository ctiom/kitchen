package kitchen

import "reflect"

var (
	typeOfISet = reflect.TypeOf((*ISet)(nil)).Elem()
)

type MenuBase[WPtr iMenu[D], D ICookware] struct {
	cookbook[D, any, any]
	name     string
	cookware D
	dishes   []iDish[D]
	path     *string
}

func InitMenu[W iMenu[D], D ICookware](menuPtr W, bundle D) W {
	menuPtr.init(menuPtr, bundle)
	return menuPtr
}

func (b *MenuBase[W, D]) init(w iMenu[D], bundle D) {
	b.initWithoutFields(w, bundle)
	b.nodes = iterateStruct(w, w, nil, bundle)
}

func iterateStruct[D ICookware](s any, parentMenu iMenu[D], set iSet[D], bundle D) []iCookbook[D] {
	var (
		fieldType reflect.StructField
		sValue    = reflect.ValueOf(s).Elem()
		sType     = sValue.Type()
		nodes     []iCookbook[D]
		path      string
	)
	for i, l := 0, sType.NumField(); i < l; i++ {
		fieldType = sType.Field(i)
		if fieldType.IsExported() && !fieldType.Anonymous {
			path = fieldType.Name
			if fieldType.Type.Implements(typeOfDish) {
				action := sValue.Field(i).Addr().Interface()
				if set != nil {
					initDish(set.(iCookbook[D]), action.(iDish[D]), path, fieldType.Tag)
				} else {
					initDish(parentMenu.(iCookbook[D]), action.(iDish[D]), path, fieldType.Tag)
				}
				nodes = append(nodes, action.(iCookbook[D]))
			} else if fieldType.Type.Implements(typeOfISet) {
				group := sValue.Field(i).Addr().Interface()
				initSet(parentMenu, group.(iSet[D]), set, path)
				nodes = append(nodes, group.(iCookbook[D]))
			} else if fieldType.Type.Implements(typeOfMenu) {
				menu := sValue.Field(i).Addr().Interface()
				InitMenu(menu.(iMenu[D]), bundle)
				nodes = append(nodes, menu.(iCookbook[D]))
			} else if fieldType.Type.Kind() == reflect.Struct {
				nodes = append(nodes, iterateStruct[D](sValue.Field(i).Addr().Interface(), parentMenu, set, bundle)...)
			}
		}
	}
	return nodes
}

func (b *MenuBase[W, D]) OverridePath(path string) *MenuBase[W, D] {
	b.path = &path
	return b
}

func (b *MenuBase[W, D]) initWithoutFields(w iMenu[D], bundle D) {
	var (
		menuValue = reflect.ValueOf(w).Elem()
		menuType  = menuValue.Type()
	)
	_, b.isTraceable = any(bundle).(ITraceableCookware)
	b.instance = w
	b.name = menuType.Name()
	_, b.isInheritableCookware = any(bundle).(ICookwareInheritable)
	_, b.isWebWrapperCookware = any(bundle).(IWebCookwareWithDataWrapper)
	w.setCookware(bundle)

	w.setName(menuType.Name())
}

func (b *MenuBase[W, D]) pushDish(action iDish[D]) int {
	b.dishes = append(b.dishes, action)
	return len(b.dishes) - 1
}

func (b *MenuBase[W, D]) Actions() []iDish[D] {
	return b.dishes
}

func (b *MenuBase[W, D]) Dishes() []iDish[D] {
	return b.dishes
}

func (b *MenuBase[W, D]) setName(name string) {
	b.name = name
}

func (b *MenuBase[W, D]) Menu() IMenu {
	return b.instance.(IMenu)
}

func (b MenuBase[W, D]) Name() string {
	if b.path != nil {
		return *b.path
	}
	return b.name
}

func (b *MenuBase[W, D]) setCookware(bundle D) {
	b.cookware = bundle
}

func (b *MenuBase[W, D]) isDataWrapper() bool {
	return b.isWebWrapperCookware
}

func (b MenuBase[W, D]) Dependency() D {
	return b.cookware
}

func (b MenuBase[W, D]) Cookware() D {
	return b.cookware
}

func (b MenuBase[W, D]) swaggerOption() SwaggerOption {
	return SwaggerOption{}
}

func (b MenuBase[W, D]) SwaggerOption(opt SwaggerOption) IMenu {
	return &menuWithSwaggerOpt{
		IMenu:  any(b.instance).(IMenu),
		option: opt,
	}
}

type menuWithSwaggerOpt struct {
	IMenu
	option SwaggerOption
}

func (w menuWithSwaggerOpt) swaggerOption() SwaggerOption {
	return w.option
}
