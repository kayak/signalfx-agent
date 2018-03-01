package metrics

import (
	"github.com/signalfx/golib/datapoint"
	"github.com/signalfx/golib/sfxclient"
	"k8s.io/api/core/v1"
)

// GAUGE(kubernetes.namespace_phase): The current phase of namespaces (`1` for
// _active_ and `0` for _terminating_)

func datapointsForNamespace(ns *v1.Namespace) []*datapoint.Datapoint {
	dims := map[string]string{
		"kubernetes_namespace": ns.Name,
	}

	return []*datapoint.Datapoint{
		sfxclient.Gauge("kubernetes.namespace_phase", dims, namespacePhaseValues[ns.Status.Phase]),
	}
}

var namespacePhaseValues = map[v1.NamespacePhase]int64{
	v1.NamespaceActive:      1,
	v1.NamespaceTerminating: 0,
	// If phase is blank for some reason, send as -1 for unknown
	v1.NamespacePhase(""): -1,
}
