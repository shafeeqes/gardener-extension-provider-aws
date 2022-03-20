// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package infrastructure

import (
	"errors"
	"fmt"

	"github.com/gardener/gardener-extension-provider-aws/pkg/apis/aws/helper"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
)

// DetermineError determines the Garden error code for the given error and creates a new error with the given message.
// TODO(shafeeqes): this is should be improved: clean up the usages to not pass the error twice (once as an error and
// once as a string) and properly wrap the given error instead of creating a new one from the given error message,
// so we can use errors.As up the call stack.
func (a *actuator) DetermineError(err error, message string) error {
	if err == nil {
		return errors.New(message)
	}

	errMsg := message
	if errMsg == "" {
		errMsg = err.Error()
	}

	codes := helper.DetermineErrorCodes(err)
	if codes == nil {
		return fmt.Errorf("%s: %+v", errMsg, err)
	}
	return gardencorev1beta1helper.NewErrorWithCodes(errMsg, codes...)
}
