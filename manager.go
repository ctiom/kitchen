package kitchen

import (
	"github.com/ctiom/kitchen/delivery"
	"errors"
	"sync"
	"sync/atomic"
)

var (
	errRunInLocal = errors.New("run in local")
)

type Manager struct {
	menus            map[string]IMenu
	menuById         []IMenu
	menuInitializers []func() IMenu
	serveMenuNames   []string
	kitchens         []IKitchen
	lock             sync.Mutex
	avgLoading       *int32
}

func NewManager() interface{} {
	var (
		avgLoading int32 = 0
	)
	return &Manager{avgLoading: &avgLoading}
}

// not select = all
func (m *Manager) SelectServeMenus(menuNames ...string) *Manager {
	m.serveMenuNames = append(m.serveMenuNames, menuNames...)
	return m
}

func (m *Manager) DisableMenuByName(name string) *Manager {
	m.lock.Lock()
	if m.menus != nil {
		if _, ok := m.menus[name]; ok {
			delete(m.menus, name)
		}
	}
	for i, n := range m.serveMenuNames {
		if n == name {
			m.serveMenuNames = append(m.serveMenuNames[:i], m.serveMenuNames[i+1:]...)
		}
	}
	m.lock.Unlock()
	return m
}

func (m *Manager) Init() {
	m.initMenus()
	m.server = delivery.NewServer()
}

func (m *Manager) initMenus() {
	m.menus = make(map[string]IMenu)
	m.menuById = make([]IMenu, len(m.menuInitializers))
	for i, menuInitializer := range m.menuInitializers {
		menu := menuInitializer()
		menu.setManager(m, uint32(i))
		m.menuById[i] = menu
		if len(m.serveMenuNames) > 0 {
			for _, name := range m.serveMenuNames {
				if menu.Name() == name {
					m.menus[name] = menu
				}
			}
		} else {
			m.menus[menu.Name()] = menu
		}
	}
}

func (m *Manager) AddMenu(menuInitializer func() IMenu) IManager {
	m.menuInitializers = append(m.menuInitializers, menuInitializer)
	return m
}

func (m *Manager) AddKitchen(kitchen IKitchen) IManager {
	m.kitchens = append(m.kitchens, kitchen)
	return m
}

func (m *Manager) UpdateLoading(loading int32) {
	for {
		avgLoading := atomic.LoadInt32(m.avgLoading)
		newAvgLoading := avgLoading + int32(float32(loading-avgLoading)/float32(len(m.kitchens)))
		if atomic.CompareAndSwapInt32(m.avgLoading, avgLoading, newAvgLoading) {
			break
		}
	}
}

func (m *Manager) Order(dish IDish, input any) (output interface{}, err error) {
	if len(m.kitchens) != 1 {
		avgLoading := atomic.LoadInt32(m.avgLoading)
		menuId := dish.Menu().ID()
		if !m.kitchens[0].canHandle(menuId, avgLoading) {
			for i, l := 1, len(m.kitchens); i < l; i++ {
				if m.kitchens[i].canHandle(menuId, avgLoading) {
					output, err = m.kitchens[i].Order(dish, input)
					if err == nil {
						return
					}
				}
			}
		}
	}
	return nil, errRunInLocal
}
