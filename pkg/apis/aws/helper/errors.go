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

package helper

import (
	"regexp"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/route53"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"k8s.io/apimachinery/pkg/util/sets"
)

// DetermineErrorCodes determines error codes based on the given error.
func DetermineErrorCodes(err error) []gardencorev1beta1.ErrorCode {
	var (
		codes   = sets.NewString()
		message = err.Error()

		unauthenticatedRegexp               = regexp.MustCompile(`(?i)(InvalidAuthenticationTokenTenant|Authentication failed|AuthFailure|invalid character|invalid_client|query returned no results|cannot fetch token|InvalidSecretAccessKey|InvalidSubscriptionId)`)
		unauthorizedRegexp                  = regexp.MustCompile(`(?i)(Unauthorized|InvalidClientTokenId|SignatureDoesNotMatch|AuthorizationFailed|invalid_grant|Authorization Profile was not found|no active subscriptions|UnauthorizedOperation|not authorized|AccessDenied|OperationNotAllowed|Error 403)`)
		quotaExceededRegexp                 = regexp.MustCompile(`(?i)((?:^|[^t]|(?:[^s]|^)t|(?:[^e]|^)st|(?:[^u]|^)est|(?:[^q]|^)uest|(?:[^e]|^)quest|(?:[^r]|^)equest)LimitExceeded|Quotas|Quota.*exceeded|exceeded quota|Quota has been met|QUOTA_EXCEEDED|Maximum number of ports exceeded|ZONE_RESOURCE_POOL_EXHAUSTED_WITH_DETAILS|VolumeSizeExceedsAvailableQuota)`)
		rateLimitsExceededRegexp            = regexp.MustCompile(`(?i)(Throttling|Too many requests)`)
		dependenciesRegexp                  = regexp.MustCompile(`(?i)(PendingVerification|Access Not Configured|accessNotConfigured|DependencyViolation|OptInRequired|DeleteConflict|Conflict|inactive billing state|ReadOnlyDisabledSubscription|is already being used|InUseSubnetCannotBeDeleted|VnetInUse|InUseRouteTableCannotBeDeleted|timeout while waiting for state to become|InvalidCidrBlock|already busy for|InsufficientFreeAddressesInSubnet|InternalServerError|internalerror|internal server error|A resource with the ID|VnetAddressSpaceCannotChangeDueToPeerings|InternalBillingError|There are not enough hosts available)`)
		retryableDependenciesRegexp         = regexp.MustCompile(`(?i)(RetryableError)`)
		resourcesDepletedRegexp             = regexp.MustCompile(`(?i)(not available in the current hardware cluster|InsufficientInstanceCapacity|SkuNotAvailable|ZonalAllocationFailed|out of stock)`)
		configurationProblemRegexp          = regexp.MustCompile(`(?i)(AzureBastionSubnet|not supported in your requested Availability Zone|InvalidParameter|InvalidParameterValue|notFound|NetcfgInvalidSubnet|InvalidSubnet|Invalid value|KubeletHasInsufficientMemory|KubeletHasDiskPressure|KubeletHasInsufficientPID|violates constraint|no attached internet gateway found|Your query returned no results|PrivateEndpointNetworkPoliciesCannotBeEnabledOnPrivateEndpointSubnet|invalid VPC attributes|PrivateLinkServiceNetworkPoliciesCannotBeEnabledOnPrivateLinkServiceSubnet|unrecognized feature gate|runtime-config invalid key|LoadBalancingRuleMustDisableSNATSinceSameFrontendIPConfigurationIsReferencedByOutboundRule|strict decoder error|not allowed to configure an unsupported|error during apply of object .* is invalid:|OverconstrainedZonalAllocationRequest|duplicate zones|overlapping zones|could not find DNS hosted zone|RRSet with DNS name [^\ ]+ is not permitted in zone [^\ ]+)`)
		retryableConfigurationProblemRegexp = regexp.MustCompile(`(?i)(is misconfigured and requires zero voluntary evictions|SDK.CanNotResolveEndpoint|The requested configuration is currently not supported)`)

		knownCodes = map[gardencorev1beta1.ErrorCode]func(string) bool{
			gardencorev1beta1.ErrorInfraUnauthenticated:          unauthenticatedRegexp.MatchString,
			gardencorev1beta1.ErrorInfraUnauthorized:             unauthorizedRegexp.MatchString,
			gardencorev1beta1.ErrorInfraQuotaExceeded:            quotaExceededRegexp.MatchString,
			gardencorev1beta1.ErrorInfraRateLimitsExceeded:       rateLimitsExceededRegexp.MatchString,
			gardencorev1beta1.ErrorInfraDependencies:             dependenciesRegexp.MatchString,
			gardencorev1beta1.ErrorRetryableInfraDependencies:    retryableDependenciesRegexp.MatchString,
			gardencorev1beta1.ErrorInfraResourcesDepleted:        resourcesDepletedRegexp.MatchString,
			gardencorev1beta1.ErrorConfigurationProblem:          configurationProblemRegexp.MatchString,
			gardencorev1beta1.ErrorRetryableConfigurationProblem: retryableConfigurationProblemRegexp.MatchString,
		}

		awsErrorCodeMap = map[gardencorev1beta1.ErrorCode]sets.String{
			gardencorev1beta1.ErrorInfraUnauthenticated: sets.NewString(
				"InvalidAccessKeyId",
				"AuthFailure",
			),
			gardencorev1beta1.ErrorInfraUnauthorized:  sets.NewString(),
			gardencorev1beta1.ErrorInfraQuotaExceeded: sets.NewString(),
			gardencorev1beta1.ErrorInfraRateLimitsExceeded: sets.NewString(
				"RequestLimitExceeded",
			),
			gardencorev1beta1.ErrorInfraDependencies:          sets.NewString(),
			gardencorev1beta1.ErrorRetryableInfraDependencies: sets.NewString(),
			gardencorev1beta1.ErrorInfraResourcesDepleted:     sets.NewString(),
			gardencorev1beta1.ErrorConfigurationProblem: sets.NewString(
				route53.ErrCodeNoSuchHostedZone,
				route53.ErrCodeInvalidChangeBatch,
			),
			gardencorev1beta1.ErrorRetryableConfigurationProblem: sets.NewString(),
		}
	)

	awsErrCode, _ := GetAWSErrorCode(err)
	if awsErrCode != "" {
		for code, errorSet := range awsErrorCodeMap {
			if !codes.Has(string(code)) && errorSet.Has(awsErrCode) {
				codes.Insert(string(code))
				// found codes don't need to be checked any more
				delete(knownCodes, code)
			}
		}
	}

	// determine error codes
	for code, matchFn := range knownCodes {
		if !codes.Has(string(code)) && matchFn(message) {
			codes.Insert(string(code))
		}
	}

	// compute error code list based on code string set
	var out []gardencorev1beta1.ErrorCode
	for _, c := range codes.List() {
		out = append(out, gardencorev1beta1.ErrorCode(c))
	}
	return out
}

func GetAWSErrorCode(err error) (awsErrCode, awsErrMsg string) {
	if aerr, ok := err.(awserr.Error); ok {
		return aerr.Code(), aerr.Message()
	}
	return "", ""
}
