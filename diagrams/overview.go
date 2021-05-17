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

func overview() {
	d, err := diagram.New(
		diagram.Filename("overview"),
		diagram.Label("Traefik Hub"),
		diagram.Direction("LR"),
		func(options *diagram.Options) {
			options.Name = "dist/overview"
		},
	)
	if err != nil {
		log.Fatal(err)
	}

	users := apps.Client.Users(diagram.NodeLabel("Users"))
	hub_agent := programming.Language.Go(diagram.NodeLabel("Traefik Hub Agent"))
	d.Group(diagram.NewGroup("Client").Label("Client").Add(
		users,
		hub_agent,
	))

	react_webapp := programming.Framework.React(diagram.NodeLabel("webapp"))
	auth0 := saas.Identity.Auth0(diagram.NodeLabel("\"sso.traefik.io\""))
	webapp_function := aws.Compute.Lambda(diagram.NodeLabel("webapp"))

	d.Group(diagram.NewGroup("Web pages").Label("Web pages").Add(
		react_webapp,
		auth0,
	))
	d.Connect(users, react_webapp)
	d.Connect(users, auth0)
	d.Connect(react_webapp, auth0)

	d.Group(diagram.NewGroup("Netlify").Label("Netlify").Add(
		webapp_function,
	))
	d.Connect(react_webapp, webapp_function)

	eks := diagram.NewGroup("AWS / EKS").Label("AWS / EKS")
	traefik := k8s.Network.Ing(diagram.NodeLabel("traefik proxy"))
	pod1 := k8s.Compute.Pod(diagram.NodeLabel("pod1"))
	podx := k8s.Compute.Pod(diagram.NodeLabel("podX"))
	eks_api := k8s.Controlplane.Api(diagram.NodeLabel("k8s API"))
	fluentbit := apps.Logging.Fluentbit(diagram.NodeLabel("Fluentbit"))
	datadog := apps.Monitoring.Datadog(diagram.NodeLabel("Datadog"))

	eks.Add(eks_api)
	eks.Group(diagram.NewGroup("Neo namespace").Label("Neo namespace").Add(
		pod1,
		podx,
	))
	eks.Group(diagram.NewGroup("Traefik namespace").Label("Traefik namespace").Add(
		traefik,
	))
	eks.Group(diagram.NewGroup("Logging namespace").Label("Loggin namespace").Add(
		fluentbit,
	))
	d.Connect(fluentbit, eks_api)
	d.Connect(fluentbit, datadog)
	d.Connect(hub_agent, traefik)
	d.Connect(webapp_function, traefik)
	eks.Connect(traefik, pod1)
	d.Group(eks)

	coll1 := apps.Database.Mongodb(diagram.NodeLabel("Collection1"))
	collx := apps.Database.Mongodb(diagram.NodeLabel("CollectionX"))
	mongo_atlas := diagram.NewGroup("MongoDB Atlas").Label("MongoDB Atlas").Add(
		coll1,
		collx,
	)
	d.Group(mongo_atlas)
	d.Connect(pod1, coll1)
	d.Connect(podx, collx)

	d.Connect(podx, oci.Monitoring.Email(diagram.NodeLabel("Sendgrid")))
	d.Connect(podx, apps.Vcs.Github(diagram.NodeLabel("GitHub")))

	if err := d.Render(); err != nil {
		log.Fatal(err)
	}
}
