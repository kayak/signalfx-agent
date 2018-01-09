package services

import (
	"reflect"

	"github.com/davecgh/go-spew/spew"
	"github.com/signalfx/neo-agent/utils"
	log "github.com/sirupsen/logrus"
)

//ID uniquely identifies a service instance
type ID string

// Endpoint is the generic interface that all types of service instances should
// implement.  All consumers of services should use this interface only.
type Endpoint interface {
	// Core returns the EndpointCore that all endpoints are required to have
	Core() *EndpointCore

	ExtraConfig() map[string]interface{}

	// Dimensions that are specific to this endpoint (e.g. container name)
	Dimensions() map[string]string
	// AddDimension adds a single dimension to the endpoint
	AddDimension(string, string)
	// RemoveDimension removes a single dimension from the endpoint
	RemoveDimension(string)
}

// HasDerivedFields is an interface with a single method that can be called to
// get fields that are derived from a service.  This is useful for things like
// aliased fields or computed fields.
type HasDerivedFields interface {
	DerivedFields() map[string]interface{}
}

// EndpointAsMap converts an endpoint to a map that contains all of the
// information about the endpoint.  This makes it easy to use endpoints in
// evaluating rules as well as in collectd templates.
func EndpointAsMap(endpoint Endpoint) map[string]interface{} {
	asMap, err := utils.ConvertToMapViaYAML(endpoint)
	if err != nil {
		log.WithFields(log.Fields{
			"error":    err,
			"endpoint": spew.Sdump(endpoint),
		}).Error("Could not convert endpoint to map")
		return nil
	}

	if asMap == nil {
		return nil
	}

	if df, ok := endpoint.(HasDerivedFields); ok {
		return utils.MergeInterfaceMaps(asMap, df.DerivedFields())
	}
	return asMap
}

// EndpointsAsSliceOfMap takes a slice of endpoint types and returns a slice of
// the result of mapping each endpoint through EndpointAsMap.  Panics if
// endpoints isn't a slice.
func EndpointsAsSliceOfMap(endpoints interface{}) []map[string]interface{} {
	val := reflect.ValueOf(endpoints)
	out := make([]map[string]interface{}, val.Len(), val.Len())
	for i := 0; i < val.Len(); i++ {
		out[i] = EndpointAsMap(val.Index(i).Interface().(Endpoint))
	}
	return out
}