package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/adamcolton/buttress/bootstrap3"
	"github.com/adamcolton/buttress/bootstrap3/bootstrap3bundle"
	"github.com/adamcolton/buttress/bootstrap3/csscontext"
	"github.com/adamcolton/buttress/html"
	"github.com/adamcolton/buttress/html/query"
	"github.com/adamcolton/fixme"
	"github.com/adamcolton/gothic/bufpool"
	"github.com/adamcolton/socketServer"
	"github.com/gorilla/websocket"
	"net/http"
	"strings"
)

var (
	mainHtmlBuf []byte
	port        = flag.String("port", ":6060", "port to run server")
)

func main() {
	flag.Parse()
	populateMainHtmlBuf()

	s := socketServer.New()
	s.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write(mainHtmlBuf)
	})
	s.HandleFunc("/fixme.js", func(w http.ResponseWriter, r *http.Request) {
		w.Write(js)
	})
	s.HandleFunc("/main.css", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "text/css")
		w.Write(css)
	})

	s.HandleWebsocket("/ws", proj)

	err := s.ListenAndServe(*port)
	println(err.Error())

}

func populateMainHtmlBuf() {
	panelBody := query.MustSelector(".panel-body")
	panelHeading := query.MustSelector(".panel-heading")
	projnameByID := query.MustSelector("#projname")

	bundle := bootstrap3bundle.New("Test UI")
	projects := bundle.Nav.Add(bootstrap3.Right, "projects", "Projects", "")
	projects.Sub("new-project", "New Project", "plus", "")
	projects.Divider()
	bundle.Nav.Add(bootstrap3.Left, "toggle", "Edit", "")
	bundle.AddScripts("/fixme.js")
	bundle.AddCSS("/main.css")
	bundle.FormStyle.Inline = true
	bundle.FormStyle.HideLabels = true

	d := bundle.Document()

	main := bundle.SinglePanel("Loading...", nil).Render()
	main.(html.TagNode).AddAttributes("id", "main-panel")

	panelBody.Query(main).AddAttributes("id", "main-body")
	panelHeading.Query(main).AddAttributes("id", "main-heading")

	projName := bundle.Form()
	projName.InputTag("text", "Project Name", "projname")
	projNameHtml := projName.Render().(html.TagNode)
	projNameHtml.AddAttributes("onsubmit", "return Comm.setName()")
	projnameByID.Query(projNameHtml).AddAttributes("onblur", "Comm.setName()")

	packageSearch := bundle.Form()
	packageSearch.InputTag("text", "Find Package", "pkgname")
	packageSearchHtml := packageSearch.Render().(html.TagNode)
	packageSearchHtml.AddAttributes("onsubmit", "return Comm.getPackages()")

	listPkgs := html.NewTag("div", "id", "pkgname-results")

	f := html.NewFragment(projNameHtml, packageSearchHtml, listPkgs)
	edit := bundle.SinglePanel("Edit", f).Render().(html.TagNode)
	edit.AddAttributes("id", "edit-panel")
	edit.AppendClass("edit")

	packages := bundle.SinglePanel("Packages", nil).Render().(html.TagNode)
	packages.AddAttributes("id", "packages-panel")
	packages.AppendClass("edit")
	panelBody.Query(packages).AddAttributes("id", "packages-body")

	delForm := bundle.Form()
	delForm.Buttons("Delete", "delete", "trash", csscontext.Danger())
	delFormHtml := delForm.Render().(html.TagNode)
	delFormHtml.AddAttributes("onsubmit", "return Comm.deletePackage()")
	del := bundle.SinglePanel("Delete", delFormHtml).Render().(html.TagNode)
	del.AppendClass("edit")

	d.AddChildren(main, edit, packages, del)

	buf := bufpool.Get()
	d.Write(buf)
	mainHtmlBuf = bufpool.PutAndCopy(buf)
}

type WSMessage struct {
	Type    string
	Package string
	Data    string
	ID      []byte
}

func proj(r *http.Request, socket *websocket.Conn) {
	close := make(chan bool)
	write := make(chan []byte, 10)

	list := fixme.List()
	listData, _ := json.Marshal(list)
	listMsg, _ := json.Marshal(WSMessage{
		Type: "list",
		Data: string(listData),
	})
	write <- listMsg

	var projMsg WSMessage
	var proj *fixme.Project
	if len(list) == 0 {
		proj = fixme.NewProject()
		proj.Save()
		projMsg.Type = "new_project"
	} else {
		proj = fixme.Load(list[0].ID)
		projMsg.Type = "load"
	}
	projMsg.Data = string(proj.JSON())
	b, _ := json.Marshal(projMsg)
	write <- b

	go func() {
		for {
			select {
			case msg := <-write:
				socket.WriteMessage(1, msg)
			case p := <-proj.Update:
				var msg WSMessage
				if p != nil {
					msg.Type = p.State().String()
					msg.Package = p.Import
					msg.Data = p.Data
				} else {
					msg.Type = "OK"
					msg.Package = "nothing to report"
					msg.Data = "Good job, buddy!"
				}
				b, _ := json.Marshal(msg)
				write <- b
			case <-close:
				return
			}
		}
	}()

	proj.ResolveDependancies()
	proj.Run()

	for data := range socketServer.ReadSocket(socket, 0) {
		var msg WSMessage
		json.Unmarshal(data, &msg)
		if h, ok := handlers[msg.Type]; ok {
			b, _ := json.Marshal(h(msg, proj))
			write <- b
		} else {
			fmt.Println("Unknown:", msg)
		}
	}

	proj.Close()
}

var handlers = map[string]func(WSMessage, *fixme.Project) WSMessage{
	"package_name":   getPackagesByName,
	"set_name":       setProjectName,
	"package_state":  setPackageState,
	"new_project":    newProject,
	"load_project":   loadProject,
	"delete_project": deleteProject,
}

func getPackagesByName(req WSMessage, p *fixme.Project) WSMessage {
	pkgs := fixme.PackageByName(req.Data)
	pkgNames := make([]string, len(pkgs))
	for i, pkg := range pkgs {
		pkgNames[i] = pkg.Import
	}
	return WSMessage{
		Type: "package_name",
		Data: strings.Join(pkgNames, "\n"),
	}

}

func setProjectName(req WSMessage, p *fixme.Project) WSMessage {
	p.Name = req.Data
	p.Save()
	return WSMessage{}
}

func setPackageState(req WSMessage, p *fixme.Project) WSMessage {
	pkg := fixme.PackageByImport(req.Package)
	if pkg == nil {
		return WSMessage{}
	}
	var action = map[string]func(*fixme.Package){
		"none":  p.Remove,
		"test":  p.AddTest,
		"lint":  p.AddLint,
		"watch": p.AddWatch,
	}
	if update, ok := action[req.Data]; ok {
		update(pkg)
		p.ResolveDependancies()
		p.DoUpdate()
	}
	return WSMessage{}
}

func newProject(req WSMessage, p *fixme.Project) WSMessage {
	p.Close()
	*p = *(fixme.NewProject())
	p.Run()
	p.DoUpdate()
	return WSMessage{
		Type: "new_project",
		Data: string(p.JSON()),
	}
}

func loadProject(req WSMessage, p *fixme.Project) WSMessage {
	*p = *(fixme.Load(req.ID))
	p.Run()
	p.ResolveDependancies()
	p.DoUpdate()
	return WSMessage{
		Type: "load",
		Data: string(p.JSON()),
	}
}

func deleteProject(req WSMessage, p *fixme.Project) WSMessage {
	p.Delete()
	return loadProject(WSMessage{}, p)
}
