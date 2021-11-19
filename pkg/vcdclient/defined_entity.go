package vcdclient

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	"github.com/vmware/cluster-api-provider-cloud-director/pkg/util"
	swagger "github.com/vmware/cluster-api-provider-cloud-director/pkg/vcdswaggerclient"
	"k8s.io/klog"
	"net/http"
	"reflect"
	"strings"
)

const (
	MaxUpdateRetries = 10
)

func (client *Client) UpdateDefinedEntityWithChanges(ctx context.Context, patch map[string]interface{}, definedEntityID string) (definedEntity *swagger.DefinedEntity, err error) {
	defer func() {
		// recover from panic if panic occurs because of
		// 1. calling Set() on a zero value
		if r := recover(); r != nil {
			err = errors.Errorf("recovered panic during updating entity: [%v]", r )
		}
	}()
	rde, resp, etag, err := client.ApiClient.DefinedEntityApi.GetDefinedEntity(ctx, definedEntityID)
	if err != nil {
		return nil, fmt.Errorf("failed to call get defined entity RDE with ID [%s]: [%s]", definedEntityID, err)
	}
	if len(patch) == 0 {
		// no updates to the entity
		return &rde, nil
	}

	if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("error getting the defined entity with ID [%s]", definedEntityID)
		return
	}

	capvcdEntity, err := util.ConvertMapToCAPVCDEntity(rde.Entity)
	if err != nil {
		err = fmt.Errorf("failed to convert map to CAPVCD entity [%v]", err)
		return
	}

	for k, v := range patch {
		fields := strings.Split(k, ".")
		updatedVal := reflect.ValueOf(v)
		klog.Infof("Assigning value ", v, " to key ", k)
		objVal := reflect.ValueOf(capvcdEntity).Elem()
		for _, attr := range fields {
			// cannot call fieldByName on a zero value
			objVal = objVal.FieldByName(attr)
			if objVal.Kind() == reflect.Ptr {
				objVal = objVal.Elem()
			}
		}
		objVal.Set(updatedVal)
	}

	// update the defined entity
	capvcdEntityMap, err := util.ConvertCAPVCDEntityToMap(capvcdEntity)
	if err != nil {
		err = fmt.Errorf("failed to convert CAPVCD entity to map: [%v]", err)
		return
	}
	rde.Entity = capvcdEntityMap
	for retries := MaxUpdateRetries ; retries > 0; retries -- {
		rde, resp, err = client.ApiClient.DefinedEntityApi.UpdateDefinedEntity(ctx, rde, etag, definedEntityID, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to update defined entity with ID [%s]: [%v]", definedEntityID, err)
		}
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("error updating the defined entity with ID [%s]. failed with status code [%d]", definedEntityID, resp.StatusCode)
		}
	}
	return &rde, nil
}
