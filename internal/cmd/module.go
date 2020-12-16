package cmd

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/hashicorp/terraform-exec/tfexec"
)

type ModuleTest struct {
	Name    string
	DirPath string

	Steps []TestStep

	output      *bytes.Buffer
	stateExists bool
	Failed      bool
}

type TestStep struct {
	Name       string
	TFVarsPath string

	output      *bytes.Buffer
	stateExists bool
	Failed      bool
}

func (module *ModuleTest) Test(ctx context.Context, tf *tfexec.Terraform) error {
	// cleanup files if anything goes wrong...
	defer module.cleanup()

	if module.output == nil {
		module.output = bytes.NewBuffer(nil)
	}

	err := tf.Init(ctx)
	if err != nil {
		module.failf("unable to terraform init: %s", err)
		return nil
	}

	if len(module.Steps) > 0 {
		for _, s := range module.Steps {
			err := s.Test(ctx, tf)
			if err != nil {
				return fmt.Errorf("unable to test module step: %w", err)
			}
			if s.stateExists {
				module.stateExists = true
			}
			if s.Failed {
				module.Failed = true
				break
			}
		}
	} else {
		// create a fake step to use instead
		s := TestStep{
			Name:   module.Name,
			output: module.output,
		}

		err := s.Test(ctx, tf)
		if err != nil {
			return fmt.Errorf("unable to test module: %w", err)
		}

		// copy step results back to module level
		module.stateExists = s.stateExists
		module.Failed = s.Failed
	}

	if module.stateExists {
		err = tf.Destroy(ctx)
		if err != nil {
			module.failf("unable to terraform destroy: %s", err)
			return nil
		}
	}

	return nil
}

func (step *TestStep) Test(ctx context.Context, tf *tfexec.Terraform) error {
	if step.output == nil {
		step.output = bytes.NewBuffer(nil)
	}

	planFile, err := filepath.Abs("testplan.tfplan")
	if err != nil {
		return fmt.Errorf("unable to locate plan file: %w", err)
	}

	changes, err := tf.Plan(ctx, tfexec.Out(planFile))
	if err != nil {
		step.failf("unable to terraform plan: %s", err)
		return nil
	}
	if !changes {
		step.logf("no changes detected during initial apply")
	}
	defer os.Remove(planFile)

	err = tf.Apply(ctx, tfexec.DirOrPlan(planFile))
	if err != nil {
		step.failf("unable to terraform apply: %s", err)
		return nil
	}

	step.stateExists = true

	// TODO: compare outputs vs expected outputs

	return nil
}

func (module *ModuleTest) cleanup() {
	os.RemoveAll(filepath.Join(module.DirPath, ".terraform"))
	os.Remove(filepath.Join(module.DirPath, "terraform.tfstate"))
	os.Remove(filepath.Join(module.DirPath, "terraform.tfstate.backup"))
	os.Remove(filepath.Join(module.DirPath, ".terraform.lock.hcl"))
	os.Remove(filepath.Join(module.DirPath, "crash.log"))
}

func (step *TestStep) failf(format string, a ...interface{}) {
	fmt.Fprintf(step.output, format, a...)
	step.Failed = true
}

func (module *ModuleTest) failf(format string, a ...interface{}) {
	fmt.Fprintf(module.output, format, a...)
	module.Failed = true
}

func (step *TestStep) logf(format string, a ...interface{}) {
	fmt.Fprintf(step.output, format, a...)
}
