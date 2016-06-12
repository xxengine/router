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

// ActionFilter check if ok to register the action to router, return
// true means the action is ok to register.
type ActionFilter func(action string) bool

// {bundle: {controller: {actions: true}}
type Actions map[string]map[string]map[string]bool

// Container stores all of the controllers
type Container struct {

	// the paths manager
	r            *Router

	controllers  map[string]func() interface{}

	// {index: bundle.controller.action}
	actions      map[int]string

	// controller comments
	comment      comment.MethodComment

	filter ActionFilter

	// self-increase action id
	index        int
}

// Param dir must be the top directory which contains all of the controllers,
// the dir param is used for read controller comments.
//
// Param filter is used to prevent some actions be registered to router, like
// the functions which extend from another struct.
func NewContainer(dir string, filter ActionFilter) *Container {

	dirs, err := helper.GetAllSubDirs(dir)
	if err != nil {
		panic(err)
	}

	// dir filter is not necessary here, so always return true.
	dirFilter := func(f os.FileInfo) bool { return true }
	_, methodComment, _, err := comment.GetDirComment(dirFilter, dirs...)
	if err != nil {
		panic(err)
	}
	return &Container{
		r: NewRouter(),
		comment: methodComment,
		controllers: make(map[string]func() interface{}, 20),
		actions: make(map[int]string, 50),
		filter: filter,
		// id cannot be 0
		index: 1,
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

// GetAllRouteMsg return all of the registered routes
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

// Register route path and controller provider to the container.
//
// Param path contains request method and url path, like "{get}/search".
// Param provider is used to provide the controller instance.
//
// [Result]:
//
// If one of the controller action(e.g. action "Country") doesn't have any route
// comments, the default route of the "Country" action would be:
//
// "{get}/search/Country"
//
// Or we can directly edit the route in "Country" action's comment:
//
// "// @route {get}/search/country"
//
// [Not suggest]:
//
// "{get|post}/path" will generate two routes "{get}/path" and "{post}/path", but I
// suggest you do not use it like that because it will make your project more complex,
// and what's the meaning of making a GET request and a POST request point to the
// same action?
func (c *Container) Add(path string, provider func() interface{}) {

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

// getCommentRoutes returns all routes from comment
func (c *Container) getCommentRoutes(comment string) []string {
	pattern := `@route {.*}[\/|\w|\:|\-]*`
	reg := regexp.MustCompile(pattern)
	strs := reg.FindAllString(comment, -1)
	return strs

	// input: "action comment @route {get|post}/login @route {put}/user"
	//
	// output: {
	// 		"@route {get|post}/login",
	// 		"@route {put}/user",
	// 	}
}

// getControllerMsg returns controller's package name, struct name and action names
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

		// ignore un-exported method
		f := []rune(action)[0]
		if 'a' <= f && f <= 'z' {
			continue
		}

		// filter actions
		if c.filter(action) {
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

	// input: "{get|post|put}/login"
	//
	// output: {"get", "post", "put"}, "login"
}
