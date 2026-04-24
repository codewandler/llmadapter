package main

import (
	"fmt"
	"strings"

	"github.com/codewandler/llmadapter/modelmeta"
	"github.com/codewandler/llmadapter/router"
	"github.com/codewandler/modeldb"
)

func resolveRouteModelDBModel(route routeConfig, endpoint router.ProviderEndpoint, catalog modeldb.Catalog, cfg modelDBConfig) (routeConfig, error) {
	if route.ModelDBModel == "" {
		return route, nil
	}
	serviceID := endpoint.Tags[tagModelDBServiceID]
	if serviceID == "" {
		return routeConfig{}, fmt.Errorf("route modeldb_model %q requires provider %q to set modeldb_service_id", route.ModelDBModel, route.Provider)
	}
	apiType, ok := modelmeta.APITypeForFamily(endpoint.Family)
	if !ok {
		return routeConfig{}, fmt.Errorf("route modeldb_model %q cannot resolve unsupported api family %q", route.ModelDBModel, endpoint.Family)
	}
	item, ok := resolveModelDBItem(catalog, cfg, serviceID, apiType, route.ModelDBModel)
	if !ok {
		return routeConfig{}, fmt.Errorf("route modeldb_model %q did not match service %q api_type %q", route.ModelDBModel, serviceID, apiType)
	}
	wireModelID := item.Offering.WireModelID
	if route.NativeModel != "" && route.NativeModel != wireModelID {
		return routeConfig{}, fmt.Errorf("route modeldb_model %q resolved to %q but native_model is %q", route.ModelDBModel, wireModelID, route.NativeModel)
	}
	if route.ModelDBWireModelID != "" && route.ModelDBWireModelID != wireModelID {
		return routeConfig{}, fmt.Errorf("route modeldb_model %q resolved to %q but modeldb_wire_model_id is %q", route.ModelDBModel, wireModelID, route.ModelDBWireModelID)
	}
	route.NativeModel = wireModelID
	route.ModelDBWireModelID = wireModelID
	return route, nil
}

func resolveModelDBItem(catalog modeldb.Catalog, cfg modelDBConfig, serviceID string, apiType modeldb.APIType, name string) (modeldb.Item, bool) {
	view := modeldb.ServiceView(catalog, serviceID, modeldb.ViewOptions{
		AliasOverlay: modelDBAliasOverlay(cfg),
		Filters: []modeldb.ItemFilter{
			func(item modeldb.Item) bool {
				return item.Offering.HasExposure(apiType)
			},
		},
	})
	if item, ok := view.Resolve(name); ok {
		return item, true
	}
	if normalized := normalizeModelDBAlias(name); normalized != name {
		if item, ok := view.Resolve(normalized); ok {
			return item, true
		}
	}
	selection, err := catalog.SelectOfferingsByModel(modeldb.ModelSelector{
		Name:      name,
		ServiceID: serviceID,
		APIType:   apiType,
	})
	if err != nil || len(selection.Offerings) == 0 {
		return modeldb.Item{}, false
	}
	offering := selection.Offerings[0]
	return modeldb.Item{
		Model:    offering.Model,
		Offering: offering.Offering,
	}, true
}

func modelDBAliasOverlay(cfg modelDBConfig) *modeldb.AliasOverlay {
	if len(cfg.Aliases) == 0 {
		return nil
	}
	out := modeldb.AliasOverlay{Bindings: make([]modeldb.AliasBinding, 0, len(cfg.Aliases))}
	for _, alias := range cfg.Aliases {
		if alias.Name == "" || alias.ServiceID == "" || alias.WireModelID == "" {
			continue
		}
		out.Bindings = append(out.Bindings, modeldb.AliasBinding{
			Name: alias.Name,
			Target: modeldb.OfferingRef{
				ServiceID:   alias.ServiceID,
				WireModelID: alias.WireModelID,
			},
		})
	}
	if len(out.Bindings) == 0 {
		return nil
	}
	return &out
}

func normalizeModelDBAlias(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	value = strings.ReplaceAll(value, "_", "-")
	value = strings.ReplaceAll(value, " ", "-")
	for strings.Contains(value, "--") {
		value = strings.ReplaceAll(value, "--", "-")
	}
	return value
}
