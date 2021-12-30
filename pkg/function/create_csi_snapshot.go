// Copyright 2022 The Kanister Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package function

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/util/rand"

	kanister "github.com/kanisterio/kanister/pkg"
	"github.com/kanisterio/kanister/pkg/kube"
	"github.com/kanisterio/kanister/pkg/kube/snapshot"
	"github.com/kanisterio/kanister/pkg/param"
)

func init() {
	_ = kanister.Register(&createCSISnapshotFunc{})
}

var (
	_ kanister.Func = (*createCSISnapshotFunc)(nil)
)

const (
	// CreateCSIVolumeSnapshotFuncName gives the name of the function
	CreateCSISnapshotFuncName = "CreateCSISnapshot"
	// CreateCSISnapshotNameArg provides name of the VolumeSnapshot to be created
	CreateCSISnapshotNameArg = "name"
	// CreateCSISnapshotPVCNameArg gives the name of the PVC to be captured
	CreateCSISnapshotPVCNameArg = "pvc"
	// CreateCSISnapshotNamespaceArg mentions the namespace of the PVC
	CreateCSISnapshotNamespaceArg = "namespace"
	// CreateCSISnapshotSnapshotClassArg specifies the name of the VolumeSnapshotClass
	CreateCSISnapshotSnapshotClassArg = "snapshotClass"
	// CreateCSISnapshotLabelsArg has labels that are to be added to the new VolumeSnapshot
	CreateCSISnapshotLabelsArg = "labels"
)

type createCSISnapshotFunc struct{}

func (*createCSISnapshotFunc) Name() string {
	return CreateCSISnapshotFuncName
}

func (*createCSISnapshotFunc) Exec(ctx context.Context, tp param.TemplateParams, args map[string]interface{}) (map[string]interface{}, error) {
	var snapshotClass *string
	var labels map[string]string
	var name, pvc, namespace string
	if err := Arg(args, CreateCSISnapshotPVCNameArg, &pvc); err != nil {
		return nil, err
	}
	if err := Arg(args, CreateCSISnapshotNamespaceArg, &namespace); err != nil {
		return nil, err
	}
	if err := Arg(args, CreateCSISnapshotSnapshotClassArg, &snapshotClass); err != nil {
		return nil, err
	}
	if err := OptArg(args, CreateCSISnapshotNameArg, &name, defaultSnapshotName(pvc, 20)); err != nil {
		return nil, err
	}
	if err := OptArg(args, CreateCSISnapshotLabelsArg, &labels, map[string]string{}); err != nil {
		return nil, err
	}

	kubeCli, err := kube.NewClient()
	if err != nil {
		return nil, err
	}
	dynCli, err := kube.NewDynamicClient()
	if err != nil {
		return nil, err
	}
	snapshotter, err := snapshot.NewSnapshotter(kubeCli, dynCli)
	if err != nil {
		return nil, err
	}
	// waitForReady is set to true by default because snapshot information is needed as output artifacts
	waitForReady := true
	if err := snapshotter.Create(ctx, name, namespace, pvc, snapshotClass, waitForReady, labels); err != nil {
		return nil, err
	}
	vs, err := snapshotter.Get(ctx, name, namespace)
	if err != nil {
		return nil, err
	}

	snapshotInfo := map[string]interface{}{
		"name":            name,
		"pvc":             pvc,
		"namespace":       namespace,
		"restoreSize":     vs.Status.RestoreSize.String(),
		"snapshotContent": vs.Status.BoundVolumeSnapshotContentName,
	}
	return snapshotInfo, nil
}

func (*createCSISnapshotFunc) RequiredArgs() []string {
	return []string{
		CreateCSISnapshotPVCNameArg,
		CreateCSISnapshotNamespaceArg,
		CreateCSISnapshotSnapshotClassArg,
	}
}

// defaultSnapshotName generates snapshot name using pvcName-snapshot-randomValue
func defaultSnapshotName(pvcName string, len int) string {
	return fmt.Sprintf("%s-snapshot-%s", pvcName, rand.String(len))
}