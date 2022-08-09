/*
Copyright 2022.

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

package results

import (
	"errors"
	"reflect"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestOperationResult(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Operation Results Test Suite")
}

var _ = BeforeSuite(func() {
	By("tearing down the test environment")
})

var _ = Describe("Operation Results", func() {

	var result *OperationResult
	var testErr = errors.New("Test")

	BeforeEach(func() {
		result = &OperationResult{}
		Expect(reflect.TypeOf(result)).To(Equal(reflect.TypeOf(&OperationResult{})))
	})

	Context("When calling ContinueProcessing", func() {
		It("returns a tuple instructing the reconcile loop to continue processing operations", func() {
			result, err := ContinueProcessing()
			Expect(err).NotTo(HaveOccurred())
			Expect(result).Should(And(
				HaveField("RequeueDelay", time.Duration(0)),
				HaveField("RequeueRequest", false),
				HaveField("CancelRequest", false),
			))
		})
	})

	Context("When calling Requeue", func() {
		It("returns a tuple instructing the reconcile loop to requeue the object", func() {
			result, err := Requeue()
			Expect(err).NotTo(HaveOccurred())
			Expect(result).Should(And(
				HaveField("RequeueDelay", time.Duration(0)),
				HaveField("RequeueRequest", true),
				HaveField("CancelRequest", false),
			))
		})
	})

	Context("When calling RequeueWithError", func() {
		It("returns a tuple instructing the reconcile loop to requeue the object with the given reconcile error", func() {
			result, err := RequeueWithError(testErr)
			Expect(err.Error()).To(Equal(testErr.Error()))
			Expect(result).Should(And(
				HaveField("RequeueDelay", time.Duration(0)),
				HaveField("RequeueRequest", true),
				HaveField("CancelRequest", false),
			))
		})
	})

	Context("When calling RequeueAfter", func() {
		It("returns a tuple instructing the reconcile loop to requeue the object after the given delay", func() {
			result, err := RequeueAfter(time.Duration(1), testErr)
			Expect(err.Error()).To(Equal(testErr.Error()))
			Expect(result).Should(And(
				HaveField("RequeueDelay", time.Duration(1)),
				HaveField("RequeueRequest", true),
				HaveField("CancelRequest", false),
			))
		})
	})

	Context("When calling RequeueOnErrorOrContinue", func() {
		It("returns a tuple instructing the reconcile loop to requeue the object in case of an error or to continue processing operations", func() {
			result, err := RequeueOnErrorOrContinue(testErr)
			Expect(err.Error()).To(Equal(testErr.Error()))
			Expect(result).Should(And(
				HaveField("RequeueDelay", time.Duration(0)),
				HaveField("RequeueRequest", false),
				HaveField("CancelRequest", false),
			))
		})
	})

	Context("When calling RequeueOnErrorOrStop", func() {
		It("returns a tuple instructing the reconcile loop to requeue the object in case of an error or to stop processing operations", func() {
			result, err := RequeueOnErrorOrStop(testErr)
			Expect(err.Error()).To(Equal(testErr.Error()))
			Expect(result).Should(And(
				HaveField("RequeueDelay", time.Duration(0)),
				HaveField("RequeueRequest", false),
				HaveField("CancelRequest", true),
			))
		})
	})

	Context("When calling StopProcessing", func() {
		It("returns a tuple instructing the reconcile loop to stop processing operations", func() {
			result, err := StopProcessing()
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(And(
				HaveField("RequeueDelay", time.Duration(0)),
				HaveField("RequeueRequest", false),
				HaveField("CancelRequest", true),
			))
		})
	})
})
