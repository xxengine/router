package router

import (
	"os"
	"reflect"
	"regexp"
	"strings"
	"path/filepath"
	"log"
	"fmt"
	"gopkg.in/orivil/comment.v0"
	"gopkg.in/orivil/sorter.v0"
	"gopkg.in/orivil/helper.v0"
)

func checkErr(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

type ActionFilter interface {
	// FilterAction for prevent register any actions to router,
	// return false means not register
	FilterAction(action string) bool
}

type Actions map[string]map[string]map[string]bool // {bundle.controller: {actions: true}}

type Container struct {
	// path manager
	r            *Router

	// controller provider container
	controllers  map[string]func() interface{}

	// contains all of the action's full name(bundle.controller.action)
	// {index: actionName}
	actions      map[int]string

	// contains all of the bundle dir's struct comment
	comment      comment.MethodComment

	// filter controller action to be registered
	actionFilter ActionFilter

	// index the actions(action id)
	index        int
}

// NewContainer 新建路由注册器并获取控制器注释
//
// dir:  包含所有控制器的最底层目录, 通过此目录的所有子文件来获取控制器的注释
// filter: 如果控制器继承了其他结构的方法, 需要将这些方法排除
func NewContainer(dir string, filter ActionFilter) *Container {
	dirFilter := func(f os.FileInfo) bool {
		// get all comment
		return true
	}
	subDirs, err := helper.GetFirstSubDirs(dir)
	checkErr(err)
	_, methodComment, _, err := comment.GetDirComment(dirFilter, subDirs...)
	checkErr(err)
	return &Container{
		r: NewRouter(),
		comment: methodComment,
		controllers: make(map[string]func() interface{}, 20),
		actions: make(map[int]string, 50),
		actionFilter: filter,
		index: 1, // id cannot be 0
	}
}

func (c *Container) addRoute(path, action string) {
	methods, path := c.getMethodsAndPath(path)
	for _, method := range methods {
		fullPath := method + path
		returnId, err := c.r.Add(fullPath, c.index)
		checkErr(err)
		if returnId != 0 {
			checkErr(fmt.Errorf("route conflict! check actions route: %s and %s\n", action, c.actions[returnId]))
		}
		c.actions[c.index] = action
		c.index++
	}
}

func (c *Container) Match(path string) (action string, params Param, controller func() interface{}, ok bool) {
	var id int
	id, params, ok = c.r.Match(path)
	if ok {
		action = c.actions[id]
		controllerName := action[0: strings.LastIndex(action, ".")]
		controller = c.controllers[controllerName]
	}
	return
}

func (c *Container) GetControllers() map[string]func() interface{} {
	return c.controllers
}

// GetAllRouteMsg get all of the route message
// only used for print
func GetAllRouteMsg(c *Container) (routeMsg []string) {
	path_id := c.r.GetAll()
	sorter := sorter.NewPrioritySorter(path_id)
	sortedPath := sorter.Sort()
	routeMsg = make([]string, len(path_id))
	paths := make([]string, len(path_id))
	actions := make([]string, len(path_id))
	index := 0
	maxPathLen := 0
	for _, path := range sortedPath {
		action := c.actions[path_id[path]]
		if len(path) > maxPathLen {
			maxPathLen = len(path)
		}
		actions[index] = action
		paths[index] = path
		index++
	}
	space := "                                                                      "
	for index, path := range paths {
		path += space[0: maxPathLen - len(path)]
		routeMsg[index] = path + " => " + actions[index]
	}
	return routeMsg
}

func (c *Container) GetActions() (actions Actions) {
	actions = make(Actions)
	for _, actionFullName := range c.actions {
		data := strings.Split(actionFullName, ".")
		bundle, controller, action := data[0], data[1], data[2]
		if actions[bundle] == nil {
			actions[bundle] = map[string]map[string]bool{controller: {action: true}}
		} else if actions[bundle][controller] == nil {
			actions[bundle][controller] = map[string]bool{action: true}
		} else {
			actions[bundle][controller][action] = true
		}
	}
	return
}

// Add for add route path and controller provider to the router container
//
// path: contains request method and url path, like "{get}/search", if the controller
// action has no route comment, result would be "{get}/search/" + "actionName"
//
// provider: the controller provider, provide the controller instance
func (c *Container) Add(path string, provider func() interface{}) {

	// reflect the controller to get actions name
	bundle, controller, actions := c.getControllerMsg(provider)

	// store the controller provider
	controllerFullName := bundle + "." + controller
	c.controllers[controllerFullName] = provider

	// add actions to router
	for _, action := range actions {
		actionFullName := controllerFullName + "." + action

		var ifAddCommentRoute = false // tag

		// get action comment
		if comment, ok := c.comment[bundle + "." + controller][action]; ok {

			// many route could match one action
			commentRoutes := c.getCommentRoutes(comment)
			if len(commentRoutes) > 0 {
				ifAddCommentRoute = true
				for _, route := range commentRoutes {
					c.addRoute(route, actionFullName)
				}
			}
		}

		// use the default action name
		if !ifAddCommentRoute {
			route := strings.TrimSuffix(path, "/") + "/" + action
			c.addRoute(route, actionFullName)
		}
	}
}

// getCommentRoutes get all routes from comment
func (c *Container) getCommentRoutes(comment string) []string {
	pattern := `@route {.*}[\/|\w|\:|\-]*`
	reg := regexp.MustCompile(pattern)
	strs := reg.FindAllString(comment, -1)
	return strs

	// in: "action comment @route {get|post}/login @route {put}/user"
	//
	// out: {
	// 		"@route {get|post}/login",
	// 		"@route {put}/user",
	// 	}
}

// getControllerMsg get controller's package name, struct name and action names
func (c *Container) getControllerMsg(provider func() interface{}) (bundle, controller string, actions []string) {
	instance := provider()
	value := reflect.ValueOf(instance)
	typ := value.Type().Elem()
	pkgPath := typ.PkgPath()
	bundle = filepath.Base(pkgPath)
	controller = typ.String()[len(bundle) + 1:]
	numMethod := value.NumMethod()
	for i := 0; i < numMethod; i++ {
		action := value.Type().Method(i).Name

		// remove unexported method
		f := []rune(action)[0]
		if 'a' <= f && f <= 'z' {
			continue
		}

		// filter actions
		if c.actionFilter.FilterAction(action) {
			actions = append(actions, action)
		}
	}
	return
}

func (c *Container) getMethodsAndPath(routePath string) (methods []string, path string) {

	pattern := `{(.*)}(.*)`
	reg := regexp.MustCompile(pattern)
	strs := reg.FindAllStringSubmatch(routePath, -1)
	methods = strings.Split(strs[0][1], "|")    // post|get => POST GET
	for index, method := range methods {
		methods[index] = strings.ToUpper(method)
	}
	path = strs[0][2]
	return

	// in: "{get|post|put}/login"
	//
	// out: {"get", "post", "put"}, "login"
}
