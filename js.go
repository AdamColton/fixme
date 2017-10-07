package main

var js = []byte(`
var UI = {
  "updatePackageState": function(evt){
    var name = evt.target.name.split("_")[0];
    Project.Active.SetPackageState(name, evt.target.value);
  },
  "addProject": function(name,id){
    UI.projectsMenu.append('<li><a id="'+id+'" onclick="Comm.setProject(event)">'+name+"</a></li>");
  }
};

$(function(){
  UI.mainBody = document.getElementById("main-body");
  UI.mainHeading = document.getElementById("main-heading");
  UI.pkgnameResults = document.getElementById("pkgname-results");
  UI.pkgname = $("#pkgname");
  UI.projname = $("#projname");
  UI.brand = $("#brand");
  UI.packagesBody = $("#packages-body");
  UI.projectsMenu = $("#projects + ul")

  var mainPanel = $("#main-heading").parent();
  var mainPanelClass = "panel-default"
  UI.setMainPanelClass = function(cls){
    mainPanel.removeClass(mainPanelClass);
    mainPanelClass = "panel-" + cls;
    mainPanel.addClass(mainPanelClass);
  };

  var editPanels = $(".edit");
  var outputPanels = $("#main-panel");

  var toggleState = "main";
  editPanels.hide();
  var toggle = $("#toggle");
  toggle.click(function(){
    if (toggleState == "edit") {
      toggleState = "main";
      editPanels.hide();
      outputPanels.show();
      toggle.html("Edit");
    } else {
      toggleState = "edit";
      editPanels.show();
      outputPanels.hide();
      toggle.html("Output");
    }
  });

  $("#new-project").click(function(){
    Comm.newProject();
  });
});

var Comm = (function(){
  function timeStr(){
    var t = new Date();
    var h = t.getHours();
    if (h<10){
      h = "0" + h
    }
    var m = t.getMinutes();
    if (m<10){
      m = "0" + m
    }
    var s = t.getSeconds();
    if (s<10){
      s = "0" + s
    }
    return h+":"+m+":"+s
  }
  var classMap = {
    "OK": "success",
    "Lint": "info",
    "Build": "danger",
    "Test": "warning",
  };
  var outputHandler = function(msg){
    UI.setMainPanelClass(classMap[msg.Type]);
    UI.mainBody.innerHTML = msg.Data;
    UI.mainHeading.innerHTML = timeStr()+") "+msg.Type +" : "+ msg.Package;
  };

  var showPackagesWithName = function(msg){
    if (msg.Data === ""){
      UI.pkgnameResults.innerHTML = "No Results";
      return;
    }
    var lines = msg.Data.split("\n");
    var i;
    for (i=0;i<lines.length;i++){
      lines[i] = Project.Active.PackageRow(lines[i], "search");
    }
    UI.pkgnameResults.innerHTML = lines.join("");
  };

  var actions = ["none", "watch", "test", "lint"];
  var loadProject = function(msg){
    var i,pkg;
    var projData = JSON.parse(msg.Data);
    Project.Active = new Project(projData.ID, projData.Name);
    UI.projname.val(Project.Active.Name);
    UI.brand.html(Project.Active.Name);
    for (i=0;i<projData.Pkgs.length;i++){
      pkg = projData.Pkgs[i];
      Project.Active.packages[pkg.Import] = actions[pkg.Action];
    }
    Project.Active.DrawPackages();
    if (msg.Type == "new_project"){
      UI.addProject(projData.Name, projData.ID);
    }
  }

  var listProjects = function(msg){
    var i,proj;
    var projects = JSON.parse(msg.Data);
    for (i=0; i<projects.length; i++){
      proj = projects[i];
      UI.addProject(proj.Name, proj.ID)
    }
  }

  var msgHandlers = {
    "OK": outputHandler,
    "Lint":outputHandler,
    "Build":outputHandler,
    "Test":outputHandler,
    "package_name": showPackagesWithName,
    "load": loadProject,
    "new_project": loadProject,
    "list": listProjects,
  };

  var conn = new WebSocket("ws://"+window.location.host+"/ws");
  conn.onmessage = function(rawMsg){
    var msg = JSON.parse(rawMsg.data);
    var handler = msgHandlers[msg.Type];
    if (handler){
      handler(msg);
    } else if (msg.Type != ""){
      console.log(msg);
    }
  }

  var send = function(type, data, package, id){
    var msg = {
      Type:type,
      Data:data,
      Package:package,
      ID: id,
    };
    conn.send(JSON.stringify(msg));
  }

  return {
    "send": send,
    "setProject": function(evt){
      send("load_project","","",evt.target.id);
    },
    "getPackages": function(){
      send("package_name", UI.pkgname.val());
      UI.pkgname.val("");
      return false;
    },
    "setName": function(){
      UI.brand.html(UI.projname.val());
      var menuItem = document.getElementById(Project.Active.ID);
      if (menuItem !== null) {
        menuItem.innerHTML = UI.projname.val();
      }
      send("set_name",UI.projname.val());
      return false; 
    },
    "newProject": function(){
      send("new_project");
    },
    "deletePackage":function(){
      document.getElementById(Project.Active.ID).parentElement.remove();
      send("delete_project");
      return false;
    }
  };
})();

function Project(id,name){
  this.Name = name;
  this.ID = id;
  this.packages = {};
}

Project.prototype.PackageRadio = function(pkg, value, text, location) {
  var sel = "";
  if (value === this.packages[pkg] || (value === "none" && this.packages[pkg] === undefined)){
    sel = 'checked="checked" ';
  }
  var radio = [
    '<input onclick="UI.updatePackageState(event)" type="radio" name="',pkg,'_',location,'" ',sel,'value="',value,'"/> ',
    text," "
  ];
  return radio.join("");
}

Project.prototype.PackageRow = function(pkg, location) {
  var row = [
    '<div class="row"><div class="col-md-6 col-lg-5">',
    this.PackageRadio(pkg,"none","None", location),
    this.PackageRadio(pkg,"watch","Watch", location),
    this.PackageRadio(pkg,"test","Test", location),
    this.PackageRadio(pkg,"lint","Lint", location),
    '</div><div class="col-md-6 col-lg-5">',
    pkg,
    '</div></div>'
  ];
  return row.join("");
}

Project.prototype.DrawPackages = function(){
  var pkgs = Object.keys(this.packages).sort();
  var html = [];
  var i, pkg;
  for(i=0;i<pkgs.length;i++){
    html.push(this.PackageRow(pkgs[i], "packages"));
  }
  UI.packagesBody.html(html.join(""));
}

Project.prototype.SetPackageState = function(pkg, value){
  if (value === "none"){
    delete this.packages[pkg];
  } else {
    this.packages[pkg] = value;
  }
  Comm.send("package_state", value, pkg);
  this.DrawPackages();
}
`)
