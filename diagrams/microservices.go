package main

import (
	"log"

	"github.com/blushft/go-diagrams/diagram"
	"github.com/blushft/go-diagrams/nodes/apps"
	"github.com/blushft/go-diagrams/nodes/aws"
	"github.com/blushft/go-diagrams/nodes/k8s"
	"github.com/blushft/go-diagrams/nodes/oci"
	"github.com/blushft/go-diagrams/nodes/programming"
	"github.com/blushft/go-diagrams/nodes/saas"
)

var (
	RESTEdgeOption = func(options *diagram.EdgeOptions) {
		options.Label = "REST"
		options.Attributes["labelfloat"] = "true"
		options.Attributes["labeldistance"] = "0.0"
		//options.Attributes["constraint"] = "false"
	}

	MongoEdgeOption = func(options *diagram.EdgeOptions) {
		options.Label = "Mongo"
		options.Attributes["labelfloat"] = "true"
		options.Attributes["labeldistance"] = "0.0"
		//options.Attributes["constraint"] = "false"
	}
)

func microservices() {
	d, err := diagram.New(
		diagram.Filename("microservices"),
		diagram.Label("Traefik Hub"),
		func(options *diagram.Options) {
			options.Name = "dist/microservices"
		},
	)
	if err != nil {
		log.Fatal(err)
	}

	hub_agent := programming.Language.Go(diagram.NodeLabel("Traefik Hub Agent"))
	webapp := aws.Compute.Lambda(diagram.NodeLabel("webapp"))
	alert := k8s.Compute.Pod(diagram.NodeLabel("alert"))
	certificates := k8s.Compute.Pod(diagram.NodeLabel("certificates"))
	cluster := k8s.Compute.Pod(diagram.NodeLabel("cluster"))
	invitation := k8s.Compute.Pod(diagram.NodeLabel("invitation"))
	metrics := k8s.Compute.Pod(diagram.NodeLabel("metrics"))
	notification := k8s.Compute.Pod(diagram.NodeLabel("notification"))
	workspace := k8s.Compute.Pod(diagram.NodeLabel("workspace"))
	token := k8s.Compute.Pod(diagram.NodeLabel("token"))
	topology := k8s.Compute.Pod(diagram.NodeLabel("topology"))
	github_proxy := k8s.Compute.Pod(diagram.NodeLabel("github_proxy"))
	traefik := k8s.Network.Ing(diagram.NodeLabel("traefik proxy"))

	d.Connect(hub_agent, traefik, RESTEdgeOption)
	d.Connect(traefik, token, RESTEdgeOption)
	d.Connect(traefik, alert, RESTEdgeOption)
	d.Connect(traefik, metrics, RESTEdgeOption)
	d.Connect(traefik, topology, RESTEdgeOption)

	d.Connect(cluster, token, RESTEdgeOption)
	d.Connect(cluster, topology, RESTEdgeOption)
	d.Connect(cluster, workspace, RESTEdgeOption)

	d.Connect(invitation, notification, RESTEdgeOption)
	d.Connect(invitation, workspace, RESTEdgeOption)

	d.Connect(alert, notification, RESTEdgeOption)
	d.Connect(alert, cluster, RESTEdgeOption)

	d.Connect(workspace, saas.Identity.Auth0(diagram.NodeLabel("\"sso.traefik.io\"")), RESTEdgeOption)

	d.Connect(topology, token, RESTEdgeOption)
	d.Connect(topology, github_proxy, RESTEdgeOption)
	d.Connect(github_proxy, apps.Vcs.Github(diagram.NodeLabel("GitHub")), RESTEdgeOption)

	d.Connect(webapp, token, RESTEdgeOption)
	d.Connect(webapp, alert, RESTEdgeOption)
	d.Connect(webapp, certificates, RESTEdgeOption)
	d.Connect(webapp, cluster, RESTEdgeOption)
	d.Connect(webapp, invitation, RESTEdgeOption)
	d.Connect(webapp, metrics, RESTEdgeOption)
	d.Connect(webapp, workspace, RESTEdgeOption)
	d.Connect(webapp, topology, RESTEdgeOption)

	d.Connect(notification, oci.Monitoring.Email(diagram.NodeLabel("Sendgrid")), RESTEdgeOption)
	d.Connect(notification, oci.Monitoring.Email(diagram.NodeLabel("WebHooks")), RESTEdgeOption)

	collMetrics := apps.Database.Mongodb(diagram.NodeLabel("metrics"))
	collAlert := apps.Database.Mongodb(diagram.NodeLabel("alert"))
	collInvitation := apps.Database.Mongodb(diagram.NodeLabel("invitation"))
	collCluster := apps.Database.Mongodb(diagram.NodeLabel("cluster"))
	collToken := apps.Database.Mongodb(diagram.NodeLabel("token"))
	collWorkspace := apps.Database.Mongodb(diagram.NodeLabel("workspace"))
	mongo_atlas := diagram.NewGroup("MongoDB Atlas").Label("MongoDB Atlas").Add(
		collMetrics,
		collAlert,
		collInvitation,
		collCluster,
		collToken,
		collWorkspace,
	)
	d.Group(mongo_atlas)
	d.Connect(metrics, collMetrics, MongoEdgeOption)
	d.Connect(alert, collAlert, MongoEdgeOption)
	d.Connect(invitation, collInvitation, MongoEdgeOption)
	d.Connect(cluster, collCluster, MongoEdgeOption)
	d.Connect(token, collToken, MongoEdgeOption)
	d.Connect(workspace, collWorkspace, MongoEdgeOption)

	if err := d.Render(); err != nil {
		log.Fatal(err)
	}
}
