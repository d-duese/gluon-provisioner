package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	// "os"
)

const (
	logFlags = log.Lshortfile | log.Ldate | log.Ltime

	listenAddress  = "127.0.0.1:8888"
	defaultDomain  = "default"
	xffHeader      = "X-Forwarded-For"
	redirectHeader = "X-Accel-Redirect"
	configFile     = "provisioner.toml"

	// nodesUrl  = "http://map.ffdo.de/data/nodes.json"
	// graphUrl  = "http://map.ffdo.de/data/graph.json"
	nodesPath = "/var/www/ffmap-d3/data_source/nodes.json"
	nodesUrl  = "http://map.freifunk-ruhrgebiet.de/data_source/nodes.json"
	graphPath = "/var/www/ffmap-d3/data_source/graph.json"
	graphUrl  = "http://map.freifunk-ruhrgebiet.de/data_source/graph.json"
	// nodesUrl = "http://map.freifunk-ruhrgebiet.de/data/nodes.json"
	// graphUrl = "http://map.freifunk-ruhrgebiet.de/data/graph.json"
)

func init() {
	log.SetFlags(logFlags)
}

func main() {
	nodeCache := NewNodeCache(60, nodesPath, nodesUrl, graphPath, graphUrl)

	http.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		defer req.Body.Close()

		setPrefix := func(prefix string) {
			w.Header().Add(redirectHeader, fmt.Sprintf("/%s%s", prefix, req.RequestURI))
		}

		remoteIp := net.ParseIP(req.Header.Get(xffHeader))
		if remoteIp == nil {
			log.Println("Cannot parse IP address in", xffHeader, "header")
			setPrefix(defaultDomain)
			return
		}

		// ToDo: persist / update in goroutine
		ndb := nodeCache.Get()
		if ndb == nil {
			log.Println(remoteIp.String(), "Node/Graph DB is empty")
			setPrefix(defaultDomain)
			return
		}

		node, ok := ndb.ips[remoteIp.String()]
		if !ok {
			log.Println(remoteIp.String(), "IP not found in alfred data")
			setPrefix(defaultDomain)
			return
		}

		move, err := node.CanBeMoved()
		if err != nil || !move {
			log.Println(remoteIp.String(), node.Nodeinfo.Hostname, "Node cannot be moved:", err)
			setPrefix(defaultDomain)
			return
		}

		domain, ignore, err := getDomain(node.Nodeinfo.Hostname)
		if err != nil {
			log.Println(remoteIp.String(), node.Nodeinfo.Hostname, "Error looking up target domain:", err)
			setPrefix(defaultDomain)
			return
		}

		if domain == "" {
			log.Println(remoteIp.String(), node.Nodeinfo.Hostname, "Node should not be moved.")
			setPrefix(defaultDomain)
			return
		} else {
			log.Println(remoteIp.String(), node.Nodeinfo.Hostname, "NODE WILL BE MOVED TO:", domain)
		}

		if ignore {
			log.Println(remoteIp.String(), node.Nodeinfo.Hostname, "Ignore flag is set, serving default firmware")
			setPrefix(defaultDomain)
		} else {
			setPrefix(domain)
		}
	})

	err := http.ListenAndServe(listenAddress, nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
	log.Println("Listening on", listenAddress)
}
