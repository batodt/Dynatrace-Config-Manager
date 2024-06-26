/**
 * @license
 * Copyright 2020 Dynatrace LLC
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package entities

import (
	"errors"
	"strings"
	"sync"

	"github.com/Dynatrace/Dynatrace-Config-Manager/one-topology/internal/idutils"
	"github.com/Dynatrace/Dynatrace-Config-Manager/one-topology/internal/log"

	"github.com/Dynatrace/Dynatrace-Config-Manager/one-topology/pkg/client"
	config "github.com/Dynatrace/Dynatrace-Config-Manager/one-topology/pkg/config/v2"
	"github.com/Dynatrace/Dynatrace-Config-Manager/one-topology/pkg/config/v2/coordinate"
	"github.com/Dynatrace/Dynatrace-Config-Manager/one-topology/pkg/config/v2/parameter"
	"github.com/Dynatrace/Dynatrace-Config-Manager/one-topology/pkg/config/v2/parameter/value"
	"github.com/Dynatrace/Dynatrace-Config-Manager/one-topology/pkg/config/v2/template"
	v2 "github.com/Dynatrace/Dynatrace-Config-Manager/one-topology/pkg/project/v2"
)

// Downloader is responsible for downloading Settings 2.0 objects
type Downloader struct {
	client client.EntitiesClient
}

// NewEntitiesDownloader creates a new downloader for Settings 2.0 objects
func NewEntitiesDownloader(c client.EntitiesClient) *Downloader {
	return &Downloader{
		client: c,
	}
}

// Download downloads all entities objects for the given entities Types

func Download(c client.EntitiesClient, specificEntitiesTypes []string, opts client.ListEntitiesOptions, projectName string) v2.ConfigsPerType {
	return NewEntitiesDownloader(c).Download(specificEntitiesTypes, opts, projectName)
}

// DownloadAll downloads all entities objects for a given project
func DownloadAll(c client.EntitiesClient, opts client.ListEntitiesOptions, projectName string) v2.ConfigsPerType {
	return NewEntitiesDownloader(c).DownloadAll(opts, projectName)
}

// Download downloads specific entities objects for the given entities Types and a given project
// The returned value is a map of entities objects with the entities Type as keys
func (d *Downloader) Download(specificEntitiesTypes []string, opts client.ListEntitiesOptions, projectName string) v2.ConfigsPerType {
	if len(specificEntitiesTypes) == 0 {
		log.Error("No Specific entity type profided for the specific-types option ")
		return nil
	}

	log.Debug("Fetching specific entities types to download")

	// get ALL entities types
	entitiesTypes, typesAsEntitiesListPtr, err := d.client.ListEntitiesTypes()
	if err != nil {
		log.Error("Failed to fetch all known entities types. Skipping entities download. Reason: %s", err)
		return nil
	}

	filteredEntitiesTypes := filterSpecificEntitiesTypes(specificEntitiesTypes, entitiesTypes)

	if filteredEntitiesTypes == nil {
		return nil
	}

	return d.download(filteredEntitiesTypes, typesAsEntitiesListPtr, opts, projectName)
}

func filterSpecificEntitiesTypes(specificEntitiesTypes []string, entitiesTypes []client.EntitiesType) []client.EntitiesType {
	filteredEntitiesTypes := make([]client.EntitiesType, 0, len(specificEntitiesTypes))

	for _, entitiesType := range entitiesTypes {
		for _, specificEntitiesType := range specificEntitiesTypes {
			if entitiesType.EntitiesTypeId == specificEntitiesType {
				filteredEntitiesTypes = append(filteredEntitiesTypes, entitiesType)
				break
			}
		}
	}

	if len(specificEntitiesTypes) != len(filteredEntitiesTypes) {
		log.Error("Did not find all provided entities Types. \n- %d Types provided: %v \n- %d Types found: %s.",
			len(specificEntitiesTypes), specificEntitiesTypes, len(filteredEntitiesTypes), filteredEntitiesTypes)
		return nil
	}

	return filteredEntitiesTypes
}

// DownloadAll downloads all entities objects for a given project.
// The returned value is a map of entities objects with the entities Type as keys
func (d *Downloader) DownloadAll(opts client.ListEntitiesOptions, projectName string) v2.ConfigsPerType {
	log.Debug("Fetching all entities types to download")

	// get ALL entities types
	entitiesTypes, typesAsEntitiesListPtr, err := d.client.ListEntitiesTypes()
	if err != nil {
		log.Error("Failed to fetch all known entities types. Skipping entities download. Reason: %s", err)
		return nil
	}

	return d.download(entitiesTypes, typesAsEntitiesListPtr, opts, projectName)
}

func (d *Downloader) download(entitiesTypes []client.EntitiesType, typesAsEntitiesListPtr *client.EntitiesList, opts client.ListEntitiesOptions, projectName string) v2.ConfigsPerType {

	results := make(v2.ConfigsPerType, len(entitiesTypes))
	downloadMutex := sync.Mutex{}
	wg := sync.WaitGroup{}
	wg.Add(len(entitiesTypes))

	for _, entitiesTypeValue := range entitiesTypes {

		go func(entityType client.EntitiesType) {
			defer wg.Done()

			entityList, err := d.client.ListEntities(entityType, opts)
			if err != nil {
				var errMsg string
				var respErr client.RespError
				if errors.As(err, &respErr) {
					errMsg = respErr.ConcurrentError()
				} else {
					errMsg = err.Error()
				}
				log.Error("Failed to fetch all entities for entities Type %s: %v", entityType.EntitiesTypeId, errMsg)
				return
			}
			if len(entityList.Entities) == 0 {
				return
			}
			log.Debug("Downloaded %d entities for entities Type %s", len(entityList.Entities), entityType.EntitiesTypeId)
			configs := d.convertObject(entityList, entityType.EntitiesTypeId, projectName)
			downloadMutex.Lock()
			results[entityType.EntitiesTypeId] = configs
			downloadMutex.Unlock()

		}(entitiesTypeValue)

	}

	wg.Wait()

	configs := d.convertObject(*typesAsEntitiesListPtr, client.TypesAsEntitiesType, projectName)
	results[client.TypesAsEntitiesType] = configs

	return results
}

func (d *Downloader) convertObject(entitiesList client.EntitiesList, entitiesType string, projectName string) []config.Config {

	content := JoinJsonElementsToArray(entitiesList.Entities)

	templ := template.NewDownloadTemplate(entitiesType, entitiesType, content)

	configId := idutils.GenerateUuidFromName(entitiesType)

	return []config.Config{{
		Template: templ,
		Coordinate: coordinate.Coordinate{
			Project:  projectName,
			Type:     entitiesType,
			ConfigId: configId,
		},
		Type: config.EntityType{
			EntitiesType: entitiesType,
			From:         entitiesList.From,
			To:           entitiesList.To,
		},
		Parameters: map[string]parameter.Parameter{
			config.NameParameter: &value.ValueParameter{Value: configId},
		},
		Skip: false,
	}}

}

func JoinJsonElementsToArray(elems []string) string {

	sep := ","
	startString := "["
	endString := "]"

	if len(elems) == 0 {
		return ""
	}

	n := len(sep) * (len(elems) - 1)
	for i := 0; i < len(elems); i++ {
		n += len(elems[i])
	}
	n += len(startString)
	n += len(endString)

	var b strings.Builder
	b.Grow(n)
	b.WriteString(startString)
	b.WriteString(elems[0])
	for _, s := range elems[1:] {
		b.WriteString(sep)
		b.WriteString(s)
	}
	b.WriteString(endString)
	return b.String()
}
