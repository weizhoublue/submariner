/*
SPDX-License-Identifier: Apache-2.0

Copyright Contributors to the Submariner project.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package datastoresyncer

import (
	"net"

	"github.com/submariner-io/admiral/pkg/log"
	"github.com/submariner-io/submariner/pkg/globalnet/constants"
	k8sv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func (d *DatastoreSyncer) handleCreateOrUpdateNode(obj runtime.Object, _ int) bool {
	node := obj.(*k8sv1.Node)
	if node.Name != d.localNodeName {
		return false
	}

	globalIPOfNode := node.GetAnnotations()[constants.SmGlobalIP]

	// Validate that globalIPOfNode falls in the globalCIDR allocated to the cluster.
	if globalIPOfNode != "" {
		_, ipnet, err := net.ParseCIDR(d.localCluster.Spec.GlobalCIDR[0])
		if err != nil {
			// Ideally this will not happen as globalCIDR is expected to be a valid CIDR.
			logger.Errorf(err, "Error parsing the GlobalCIDR %q", d.localCluster.Spec.GlobalCIDR)
			return false
		}

		if ipnet.Contains(net.ParseIP(globalIPOfNode)) {
			return d.updateLocalEndpointIfNecessary(globalIPOfNode)
		}
	}

	return false
}

func (d *DatastoreSyncer) areNodesEquivalent(obj1, obj2 *unstructured.Unstructured) bool {
	if obj1.GetName() != d.localNodeName {
		// Ignore this event. We are only interested in active GatewayNode events.
		return true
	}

	existingGlobalIP := obj1.GetAnnotations()[constants.SmGlobalIP]
	newGlobalIP := obj2.GetAnnotations()[constants.SmGlobalIP]

	logger.V(log.TRACE).Infof("areNodesEquivalent called for %q, existingGlobalIP %q, newGlobalIP %q",
		obj1.GetName(), existingGlobalIP, newGlobalIP)

	return existingGlobalIP == newGlobalIP
}

func (d *DatastoreSyncer) updateLocalEndpointIfNecessary(globalIPOfNode string) bool {
	if d.localEndpoint.Spec.HealthCheckIP != globalIPOfNode {
		logger.Infof("Updating the endpoint HealthCheckIP to globalIP %q", globalIPOfNode)

		prevHealthCheckIP := d.localEndpoint.Spec.HealthCheckIP
		d.localEndpoint.Spec.HealthCheckIP = globalIPOfNode

		if err := d.createOrUpdateLocalEndpoint(); err != nil {
			logger.Warningf("Error updating the local submariner Endpoint with HealthcheckIP: %v", err)

			d.localEndpoint.Spec.HealthCheckIP = prevHealthCheckIP

			return true
		}
	}

	return false
}
