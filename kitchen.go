package kitchen

import "net"

type kitchen struct {
	addr              string
	conn              net.Conn
	availableMenus    []bool
	manager           IManager
	loadingAdjustment int32
	loading           int32
}

func (k kitchen) Order(dish IDish, input any) (output interface{}, err error) {
	//TODO implement me
	panic("implement me")
}

func (k kitchen) canHandle(menuId uint32, avgLoading int32) bool {
	return uint32(len(k.availableMenus)) > menuId && k.availableMenus[menuId] && k.loading*k.loadingAdjustment < avgLoading
}
