// Copyright 2016 orivil Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// Package router provides a container for storing controllers, also provided
// a route matcher to configure the route and match an incoming request path.
package router
import (
	"strings"
	"fmt"
	"strconv"
)

const (
	// param start with
	wildcard  = "::"
)

type trie struct {
	id int
	param string
	next map[string]*trie
}

type Router struct {
	routes *trie
}

type Param map[string]string

func (p Param)GetInt(key string) (int, error) {
	return strconv.Atoi(p[key])
}

func NewRouter() *Router {
	return &Router{routes: new(trie)}
}

func (this *Router) Match(path string) (id int, ps Param, matched bool) {
	currentNode := this.routes
	routes := splitPath(path)
	matched = true
	for _, r := range routes {
		if nextNode, ok := currentNode.next[r]; ok {
			currentNode = nextNode
		} else if nextNode, ok := currentNode.next[wildcard]; ok {
			currentNode = nextNode
			if ps == nil {
				ps = Param(map[string]string{currentNode.param: r})
			} else {
				ps[currentNode.param] = r
			}
		} else {
			matched = false
			break
		}
	}
	id = currentNode.id
	// matched all node but there's no action id
	if id == 0 {
		matched = false
	}
	return
}

func (this *Router) getParam(nodePath string) (param string, contain bool) {
	contain = strings.HasPrefix(nodePath, "::")
	if contain {
		param = strings.TrimPrefix(nodePath, "::")
	}
	return
}

// Add is used to register path to trie, the function will return the last node's
// stored id which the path matched, if the last node's stored id is not 0, means
// this node has already registered a route and will return the registered id.
func (this *Router) Add(path string, id int) (existId int, e error) {
	currentNode := this.routes
	routes := splitPath(path)
	for _, r := range routes {
		_param := ""
		if p, ok := this.getParam(r); ok {
			r = wildcard
			_param = p
		}
		if currentNode.next == nil {
			newNode := &trie{param: _param}
			currentNode.next = map[string]*trie{r: newNode}
			currentNode = newNode
		} else if _currentNode, ok := currentNode.next[r]; ok {
			if _currentNode.param != "" && _param != "" && _currentNode.param != _param {
				e = fmt.Errorf(
					"route path <%s> got error: the parameter <%s> must be named <%s>",
					path,
					_param,
					_currentNode.param,
				)
				return
			} else {
				currentNode = _currentNode
			}
		} else {
			newNode := &trie{param: _param}
			currentNode.next[r] = newNode
			currentNode = newNode
		}
	}
	existId = currentNode.id
	currentNode.id = id
	return
}

// GetAll returns all of the registered routes and the controller id.
func (this *Router) GetAll() map[string]int {
	return getNextAllPath("", this.routes.next)
}

func getNextAllPath(path string, nexts map[string]*trie) (paths map[string]int) {
	paths = make(map[string]int, 1)
	for p, t := range nexts {
		if p == wildcard {
			p = path + "/" + wildcard + t.param
		} else {
			p = path + "/" + p
		}
		if t.id != 0 {
			paths[p] = t.id
		}

		if t.next != nil {
			nextPaths := getNextAllPath(p, t.next)
			for path, id := range nextPaths {
				paths[path] = id
			}
		}
	}
	return
}

func splitPath(path string) (paths []string) {
	path = strings.TrimSuffix(path, "/")
	path = strings.TrimPrefix(path, "/")
	paths = strings.Split(path, "/")
	return
}