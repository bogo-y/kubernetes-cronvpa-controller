package observe

import (
	"github.com/bogo-y/kubernetes-cronvpa-controller/internal/controller"
	"github.com/bogo-y/kubernetes-cronvpa-controller/internal/cronjob"
	"github.com/gorilla/mux"
	"html/template"
	"k8s.io/klog/v2"
	"net/http"
	"time"
)

type WebServer struct {
	manager *controller.Manager
}

type data struct {
	Items []Item
}
type Item struct {
	ID        string
	Namespace string
	targetRef string
	Pre       string
	Nxt       string
}

func (ws *WebServer) handleIndexController(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	tmpl, _ := template.New("index").Parse(Template)
	entries := ws.manager.Executor.ListJob()
	d := data{
		Items: make([]Item, 0),
	}
	for _, e := range entries {
		_, ok := e.Job.(cronjob.CronJob)
		if !ok {
			klog.Warningf("Failed to parse cronjob %v to web console", e)
			continue
		}
		d.Items = append(d.Items, Item{
			ID:  e.Job.ID(),
			Pre: e.Prev.String(),
			Nxt: e.Next.String(),
		})
	}
	tmpl.Execute(w, d)
}
func (ws *WebServer) Serve() {
	r := mux.NewRouter()
	r.HandleFunc("/", ws.handleIndexController)
	http.Handle("/", r)
	srv := &http.Server{
		Handler:      r,
		Addr:         ":8080",
		WriteTimeout: 20 * time.Second,
		ReadTimeout:  20 * time.Second,
	}
	klog.Fatal(srv.ListenAndServe())
}

func NewWebServer(m *controller.Manager) *WebServer {
	return &WebServer{manager: m}
}
