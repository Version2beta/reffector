// Package other shares the dependency dispatch with the fixture's root package.
package other

import (
	"example.com/dependency"
	"example.com/waitgroupfixture"
)

func EmptyRoot() { dependency.Dispatch(waitgroupfixture.Empty{}) }

func WaiterRoot() { dependency.Dispatch(waitgroupfixture.Waiter{}) }
