// Copyright 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package worker

import (
	"context"

	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	extensionspredicate "github.com/gardener/gardener/extensions/pkg/predicate"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils/mapper"
)

const (
	// FinalizerName is the worker controller finalizer.
	FinalizerName = "extensions.gardener.cloud/worker"
	// ControllerName is the name of the controller.
	ControllerName = "worker"
	// ControllerNameState is the name of the controller responsible for updating the worker's state.
	ControllerNameState = "worker-state"
)

// AddArgs are arguments for adding an worker controller to a manager.
type AddArgs struct {
	// Actuator is an worker actuator.
	Actuator Actuator
	// ControllerOptions are the controller options used for creating a controller.
	// The options.Reconciler is always overridden with a reconciler created from the
	// given actuator.
	ControllerOptions controller.Options
	// Predicates are the predicates to use.
	// If unset, GenerationChangedPredicate will be used.
	Predicates []predicate.Predicate
	// Type is the type of the resource considered for reconciliation.
	Type string
	// IgnoreOperationAnnotation specifies whether to ignore the operation annotation or not.
	// If the annotation is not ignored, the extension controller will only reconcile
	// with a present operation annotation typically set during a reconcile (e.g in the maintenance time) by the Gardenlet
	IgnoreOperationAnnotation bool
}

// DefaultPredicates returns the default predicates for a Worker reconciler.
func DefaultPredicates(ctx context.Context, mgr manager.Manager, ignoreOperationAnnotation bool) []predicate.Predicate {
	return extensionspredicate.DefaultControllerPredicates(ignoreOperationAnnotation, extensionspredicate.ShootNotFailedPredicate(ctx, mgr))
}

// Add creates a new Worker Controller and adds it to the Manager.
// and Start it when the Manager is Started.
func Add(ctx context.Context, mgr manager.Manager, args AddArgs) error {
	args.ControllerOptions.Reconciler = NewReconciler(mgr, args.Actuator)

	predicates := extensionspredicate.AddTypePredicate(args.Predicates, args.Type)
	if err := add(ctx, mgr, args, predicates); err != nil {
		return err
	}

	return addStateUpdatingController(ctx, mgr, args.ControllerOptions, args.Type)
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(ctx context.Context, mgr manager.Manager, args AddArgs, predicates []predicate.Predicate) error {
	ctrl, err := controller.New(ControllerName, mgr, args.ControllerOptions)
	if err != nil {
		return err
	}

	if args.IgnoreOperationAnnotation {
		if err := ctrl.Watch(
			&source.Kind{Type: &extensionsv1alpha1.Cluster{}},
			mapper.EnqueueRequestsFrom(ctx, mgr.GetCache(), ClusterToWorkerMapper(ctx, mgr, predicates), mapper.UpdateWithNew, ctrl.GetLogger()),
		); err != nil {
			return err
		}
	}

	return ctrl.Watch(&source.Kind{Type: &extensionsv1alpha1.Worker{}}, &handler.EnqueueRequestForObject{}, predicates...)
}

func addStateUpdatingController(ctx context.Context, mgr manager.Manager, options controller.Options, extensionType string) error {
	var (
		machinePredicates = []predicate.Predicate{
			predicate.Or(
				MachineNodeInfoHasChanged(),
				predicate.GenerationChangedPredicate{},
			),
		}
		workerPredicates = []predicate.Predicate{
			extensionspredicate.HasType(extensionType),
		}
	)

	ctrl, err := controller.New(ControllerNameState, mgr, controller.Options{
		MaxConcurrentReconciles: options.MaxConcurrentReconciles,
		Reconciler:              NewStateReconciler(mgr),
	})
	if err != nil {
		return err
	}

	if err := ctrl.Watch(
		&source.Kind{Type: &machinev1alpha1.MachineSet{}},
		mapper.EnqueueRequestsFrom(ctx, mgr.GetCache(), MachineSetToWorkerMapper(workerPredicates), mapper.UpdateWithNew, ctrl.GetLogger()),
		machinePredicates...,
	); err != nil {
		return err
	}

	return ctrl.Watch(
		&source.Kind{Type: &machinev1alpha1.Machine{}},
		mapper.EnqueueRequestsFrom(ctx, mgr.GetCache(), MachineToWorkerMapper(workerPredicates), mapper.UpdateWithNew, ctrl.GetLogger()),
		machinePredicates...,
	)
}
