/*
Copyright 2020 The Kubernetes Authors.

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

package bootstrap

// IAM action and service name constants shared across bootstrap policy files.
const (
	iamActionEC2AssignIPv6Addresses     = "ec2:AssignIpv6Addresses"
	iamActionEC2DescribeInstances       = "ec2:DescribeInstances"
	iamActionEC2DescribeVolumes         = "ec2:DescribeVolumes"
	iamActionEC2CreateTags              = "ec2:CreateTags"
	iamActionEC2DescribeTags            = "ec2:DescribeTags"
	iamActionEC2DescribeInstanceTypes   = "ec2:DescribeInstanceTypes"
	iamActionIAMCreateServiceLinkedRole = "iam:CreateServiceLinkedRole"
	iamConditionAWSServiceName          = "iam:AWSServiceName"
	iamActionSSMGetParameter            = "ssm:GetParameter"
	iamServiceEKS                       = "eks.amazonaws.com"
)
