package kitchen

import (
	"github.com/iancoleman/strcase"
	"net/http"
	"strings"
)

// Deprecated: Use ToRouterV2 instead
func (w cookbook[D, I, O]) ToRouter(prefix ...string) map[string]map[string]http.HandlerFunc {
	var (
		res         = map[string]map[string]http.HandlerFunc{}
		url, method string
		urlParts    []string
		handlers    map[string]http.HandlerFunc
		handler     http.HandlerFunc
		ok          bool
		group       iSet[D]
		action      iDish[D]
	)
	if len(prefix) != 0 {
		if prefix[0] == "" {
			prefix = prefix[1:]
		} else if strings.HasPrefix(prefix[0], "/") {
			prefix[0] = prefix[0][1:]
		}
	}
	for _, node := range w.nodes {
		if group, ok = any(node).(iSet[D]); ok {
			for method, handlers = range node.ToRouter(append(prefix, strcase.ToSnake(group.Name()))...) {
				if _, ok = res[method]; !ok {
					res[method] = map[string]http.HandlerFunc{}
				}
				for url, handler = range handlers {
					res[method][url] = handler
				}
			}
		} else if action, ok = any(node).(iDish[D]); ok {
			method, urlParts, handler = action.ServeHttp()
			urlParts = append(prefix, urlParts...)
			if _, ok = res[method]; !ok {
				res[method] = map[string]http.HandlerFunc{}
			}
			res[method][strings.ReplaceAll("/"+strings.Join(urlParts, "/"), "//", "/")] = handler
		}
	}
	return res
}

type HandlerFuncWithUrl struct {
	Url string
	http.HandlerFunc
}

func (w cookbook[D, I, O]) ToRouterV2(prefix ...string) map[string][]HandlerFuncWithUrl {
	var (
		res         = map[string][]HandlerFuncWithUrl{}
		url, method string
		urlParts    []string
		handlers    map[string]http.HandlerFunc
		handler     http.HandlerFunc
		ok          bool
		group       iSet[D]
		action      iDish[D]
	)
	if len(prefix) != 0 {
		if prefix[0] == "" {
			prefix = prefix[1:]
		} else if strings.HasPrefix(prefix[0], "/") {
			prefix[0] = prefix[0][1:]
		}
	}
	for _, node := range w.nodes {
		if group, ok = any(node).(iSet[D]); ok {
			for method, handlers = range node.ToRouter(append(prefix, strcase.ToSnake(group.Name()))...) {
				if _, ok = res[method]; !ok {
					res[method] = []HandlerFuncWithUrl{}
				}
				for url, handler = range handlers {
					res[method] = append(res[method], HandlerFuncWithUrl{url, handler})
				}
			}
		} else if action, ok = any(node).(iDish[D]); ok {
			method, urlParts, handler = action.ServeHttp()
			urlParts = append(prefix, urlParts...)
			if _, ok = res[method]; !ok {
				res[method] = []HandlerFuncWithUrl{}
			}
			res[method] = append(res[method], HandlerFuncWithUrl{strings.ReplaceAll("/"+strings.Join(urlParts, "/"), "//", "/"), handler})
		}
	}
	return res
}
