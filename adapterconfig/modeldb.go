package adapterconfig

import (
	"fmt"
	"sort"
	"strings"

	"github.com/codewandler/llmadapter/modelmeta"
	"github.com/codewandler/llmadapter/router"
	"github.com/codewandler/modeldb"
)

func modelDBCatalog(cfg Config) (modeldb.Catalog, bool, error) {
	if !ConfigUsesModelDB(cfg) {
		return modeldb.Catalog{}, false, nil
	}
	catalog, err := LoadModelDBCatalog(cfg.ModelDB)
	if err != nil {
		return modeldb.Catalog{}, false, err
	}
	return catalog, true, nil
}

func ConfigUsesModelDB(cfg Config) bool {
	if cfg.ModelDB.CatalogPath != "" || len(cfg.ModelDB.OverlayPaths) != 0 || len(cfg.ModelDB.Aliases) != 0 {
		return true
	}
	for _, route := range cfg.Routes {
		if route.ModelDBModel != "" {
			return true
		}
		if pricingWireModel(route) != "" || route.DynamicModels {
			for _, provider := range cfg.Providers {
				if providerMatchesRoute(provider, route) && providerModelDBServiceID(provider) != "" {
					return true
				}
			}
		}
	}
	return false
}

func LoadModelDBCatalog(cfg ModelDBConfig) (modeldb.Catalog, error) {
	var (
		catalog modeldb.Catalog
		err     error
	)
	if cfg.CatalogPath != "" {
		catalog, err = modeldb.LoadJSON(cfg.CatalogPath)
		if err != nil {
			return modeldb.Catalog{}, fmt.Errorf("load modeldb catalog %q: %w", cfg.CatalogPath, err)
		}
	} else {
		catalog, err = modeldb.LoadBuiltIn()
		if err != nil {
			return modeldb.Catalog{}, fmt.Errorf("load built-in modeldb catalog: %w", err)
		}
	}
	for _, path := range cfg.OverlayPaths {
		overlay, err := modeldb.LoadJSON(path)
		if err != nil {
			return modeldb.Catalog{}, fmt.Errorf("load modeldb overlay %q: %w", path, err)
		}
		if err := mergeCatalog(&catalog, overlay); err != nil {
			return modeldb.Catalog{}, fmt.Errorf("merge modeldb overlay %q: %w", path, err)
		}
	}
	return catalog, nil
}

func resolveRouteModelDBModel(route RouteConfig, endpoint router.ProviderEndpoint, catalog modeldb.Catalog, cfg ModelDBConfig) (RouteConfig, error) {
	if route.ModelDBModel == "" {
		return route, nil
	}
	serviceID := endpoint.Tags[TagModelDBServiceID]
	if serviceID == "" {
		return RouteConfig{}, fmt.Errorf("route modeldb_model %q requires provider %q to set modeldb_service_id", route.ModelDBModel, route.Provider)
	}
	apiType, ok := modelmeta.APITypeForFamily(endpoint.Family)
	if !ok {
		return RouteConfig{}, fmt.Errorf("route modeldb_model %q cannot resolve unsupported api family %q", route.ModelDBModel, endpoint.Family)
	}
	item, ok := resolveModelDBItem(catalog, cfg, serviceID, apiType, route.ModelDBModel)
	if !ok {
		return RouteConfig{}, fmt.Errorf("route modeldb_model %q did not match service %q api_type %q", route.ModelDBModel, serviceID, apiType)
	}
	wireModelID := item.Offering.WireModelID
	if route.NativeModel != "" && route.NativeModel != wireModelID {
		return RouteConfig{}, fmt.Errorf("route modeldb_model %q resolved to %q but native_model is %q", route.ModelDBModel, wireModelID, route.NativeModel)
	}
	if route.ModelDBWireModelID != "" && route.ModelDBWireModelID != wireModelID {
		return RouteConfig{}, fmt.Errorf("route modeldb_model %q resolved to %q but modeldb_wire_model_id is %q", route.ModelDBModel, wireModelID, route.ModelDBWireModelID)
	}
	route.NativeModel = wireModelID
	route.ModelDBWireModelID = wireModelID
	return route, nil
}

func resolveModelDBItem(catalog modeldb.Catalog, cfg ModelDBConfig, serviceID string, apiType modeldb.APIType, name string) (modeldb.Item, bool) {
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
	return modeldb.Item{Model: offering.Model, Offering: offering.Offering}, true
}

func modelDBAliasOverlay(cfg ModelDBConfig) *modeldb.AliasOverlay {
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

func mergeCatalog(dst *modeldb.Catalog, src modeldb.Catalog) error {
	return modeldb.MergeCatalogFragment(dst, &modeldb.Fragment{
		Services:  catalogServices(src),
		Models:    catalogModels(src),
		Offerings: catalogOfferings(src),
	})
}

func catalogServices(catalog modeldb.Catalog) []modeldb.Service {
	out := make([]modeldb.Service, 0, len(catalog.Services))
	for _, service := range catalog.Services {
		out = append(out, service)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func catalogModels(catalog modeldb.Catalog) []modeldb.ModelRecord {
	out := make([]modeldb.ModelRecord, 0, len(catalog.Models))
	for _, model := range catalog.Models {
		out = append(out, model)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Key.Creator != out[j].Key.Creator {
			return out[i].Key.Creator < out[j].Key.Creator
		}
		if out[i].Key.Family != out[j].Key.Family {
			return out[i].Key.Family < out[j].Key.Family
		}
		if out[i].Key.Series != out[j].Key.Series {
			return out[i].Key.Series < out[j].Key.Series
		}
		if out[i].Key.Version != out[j].Key.Version {
			return out[i].Key.Version < out[j].Key.Version
		}
		if out[i].Key.Variant != out[j].Key.Variant {
			return out[i].Key.Variant < out[j].Key.Variant
		}
		return out[i].Key.ReleaseDate < out[j].Key.ReleaseDate
	})
	return out
}

func catalogOfferings(catalog modeldb.Catalog) []modeldb.Offering {
	out := make([]modeldb.Offering, 0, len(catalog.Offerings))
	for _, offering := range catalog.Offerings {
		out = append(out, offering)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].ServiceID != out[j].ServiceID {
			return out[i].ServiceID < out[j].ServiceID
		}
		return out[i].WireModelID < out[j].WireModelID
	})
	return out
}
