// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"github.com/hashicorp/terraform-plugin-framework/function"
	"strings"
)

var _ function.Function = &ICaseAsgIdFunction{}

type ICaseAsgIdFunction struct{}

func NewICaseAsgIdFunction() function.Function {
	return &ICaseAsgIdFunction{}
}

func (f *ICaseAsgIdFunction) Metadata(ctx context.Context, req function.MetadataRequest, resp *function.MetadataResponse) {
	resp.Name = "icase_asgid"
}

func (f *ICaseAsgIdFunction) Definition(ctx context.Context, req function.DefinitionRequest, resp *function.DefinitionResponse) {
	resp.Definition = function.Definition{
		Summary:     "Generate Azure ASG id with upper or lower case resource group and ASG name",
		Description: "Work around Azure ASG case plague",
		Parameters: []function.Parameter{
			function.BoolParameter{
				Name:        "uppercase",
				Description: "when true, switch RG and ASG to uppercase",
			},
			function.StringParameter{
				Name:        "asgid",
				Description: "the asg identifier",
			},
		},
		Return: function.StringReturn{},
	}
}

func (f *ICaseAsgIdFunction) Run(
	ctx context.Context,
	req function.RunRequest,
	resp *function.RunResponse,
) {
	var upper bool
	var input string

	output := ""

	resp.Error = function.ConcatFuncErrors(req.Arguments.Get(ctx, &upper, &input))
	if !upper {
		resp.Error = function.ConcatFuncErrors(resp.Error, resp.Result.Set(ctx, input))
		return
	}
	parts := strings.Split(input, "/")
	if len(parts) != 9 {
		resp.Error = function.ConcatFuncErrors(function.NewFuncError("invalid ASG id, expecting 8 parts"))
	} else {
		parts[4] = strings.ToUpper(parts[4])
		parts[8] = strings.ToUpper(parts[8])
		output = strings.Join(parts, "/")
	}
	resp.Error = function.ConcatFuncErrors(resp.Error, resp.Result.Set(ctx, output))
}
