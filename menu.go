package kitchen

import (
	"context"
	"errors"
	"github.com/go-preform/kitchen/delivery"
	"reflect"
	"sync"
)

var (
	typeOfISet = reflect.TypeOf((*ISet)(nil)).Elem()
)

type MenuBase[WPtr iMenu[D], D ICookware] struct {
	cookbook[D, any, any]
	name           string
	menuCookware   D
	dishes         []iDish[D]
	dishCnt        uint32
	path           *string
	manager        IManager
	idUnderManager uint32
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
		ok        bool
	)
	for i, l := 0, sType.NumField(); i < l; i++ {
		fieldType = sType.Field(i)
		if fieldType.IsExported() && !fieldType.Anonymous {
			path = fieldType.Name
			node := sValue.Field(i).Addr().Interface()
			if _, ok = node.(IDish); ok {
				if set != nil {
					initDish(set.(iCookbook[D]), node.(iDish[D]), path, fieldType.Tag)
				} else {
					initDish(parentMenu.(iCookbook[D]), node.(iDish[D]), path, fieldType.Tag)
				}
				nodes = append(nodes, node.(iCookbook[D]))
			} else if _, ok = node.(ISet); ok {
				initSet(parentMenu, node.(iSet[D]), set, path)
				nodes = append(nodes, node.(iCookbook[D]))
			} else if _, ok = node.(IMenu); ok {
				InitMenu(node.(iMenu[D]), bundle)
				nodes = append(nodes, node.(iCookbook[D]))
			} else if fieldType.Type.Kind() == reflect.Struct {
				nodes = append(nodes, iterateStruct[D](sValue.Field(i).Addr().Interface(), parentMenu, set, bundle)...)
			}
		}
	}
	return nodes
}

func (r MenuBase[W, D]) Cookware() ICookware {
	return r.menuCookware
}

func (b *MenuBase[W, D]) Manager() IManager {
	return b.manager
}

func (b *MenuBase[W, D]) ID() uint32 {
	return b.idUnderManager
}

func (b *MenuBase[W, D]) setManager(m IManager, id uint32) {
	b.idUnderManager = id
	b.manager = m.(*Manager)
	for _, d := range b.dishes {
		d.refreshCooker()
	}
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
	b.concurrentLimit = new(int32)
	b.running = new(int32)
	b.locker = &sync.Mutex{}
	w.setCookware(bundle)

	w.setName(menuType.Name())
}

func (b *MenuBase[W, D]) pushDish(action iDish[D]) int {
	b.dishes = append(b.dishes, action)
	b.dishCnt++
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

func (b *MenuBase[W, D]) setCookware(cookware D) {
	b.menuCookware = cookware
}

func (b *MenuBase[W, D]) isDataWrapper() bool {
	return b.isWebWrapperCookware
}

func (b MenuBase[W, D]) Dependency() D {
	return b.menuCookware
}

func (b MenuBase[W, D]) cookware() D {
	return b.menuCookware
}

var errDishNotFound = errors.New("dish not found")

func (b MenuBase[W, D]) orderDish(ctx context.Context, order *delivery.Order) {
	if order.DishId >= b.dishCnt {
		_ = order.Response(nil, errDishNotFound)
		return
	}
	output, err := b.dishes[order.DishId].cookByte(ctx, order.Input)
	_ = order.Response(output, err)
	return
}
