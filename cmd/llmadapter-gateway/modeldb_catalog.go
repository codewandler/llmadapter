package main

import (
	"sort"

	"github.com/codewandler/modeldb"
)

func catalogServices(catalog modeldb.Catalog) []modeldb.Service {
	out := make([]modeldb.Service, 0, len(catalog.Services))
	for _, service := range catalog.Services {
		out = append(out, service)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})
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
