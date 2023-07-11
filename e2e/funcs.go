package e2e

import (
	"context"
	"os"
	"os/exec"
	"sync"

	"github.com/takutakahashi/oci-image-operator/actor/base/pkg/base"
	buildv1beta1 "github.com/takutakahashi/oci-image-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1apply "k8s.io/client-go/applyconfigurations/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

func prepare(ctx context.Context) client.Client {
	if os.Getenv("E2E_CONTEXT") == "" {
		panic("set E2E_CONTEXT")
	}
	_, err := exec.CommandContext(ctx, "kubectx", os.Getenv("E2E_CONTEXT")).Output()
	iferr(err)
	_, err = exec.CommandContext(ctx, "make", "-C", "..", "deploy", "IMG=localhost:5000/controller").Output()
	iferr(err)
	cfg, err := config.GetConfigWithContext(os.Getenv("E2E_CONTEXT"))
	iferr(err)
	c, err := base.GenClient(cfg)
	if err != nil {
		panic(err)
	}
	imt := newImageFlowTemplate("test")
	image := newImage("test")
	c.Create(ctx, imt, &client.CreateOptions{})
	c.Create(ctx, newSecret("test"), &client.CreateOptions{})
	c.Create(ctx, image, &client.CreateOptions{})
	//
	return c
}

func teardown(c client.Client) {
	ctx := context.Background()
	imt := newImageFlowTemplate("test")
	image := newImage("test")
	iferr(c.Delete(ctx, image, &client.DeleteOptions{}))
	iferr(c.Delete(ctx, imt, &client.DeleteOptions{}))
	iferr(c.Delete(ctx, newSecret("test"), &client.DeleteOptions{}))
	iferr(c.Delete(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "oci-image-operator-system",
		},
	}))
}
func iferr(err error) {
	if err != nil {
		panic(err)
	}
}

func newImage(name string) *buildv1beta1.Image {
	return &buildv1beta1.Image{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		Spec: buildv1beta1.ImageSpec{
			TemplateName: "test",
			Repository: buildv1beta1.ImageRepository{
				URL: "https://github.com/taktuakahashi/build-test.git",
				TagPolicies: []buildv1beta1.ImageTagPolicy{
					{
						Policy:   buildv1beta1.ImageTagPolicyTypeBranchHash,
						Revision: "e2e",
					},
				},
			},
			Targets: []buildv1beta1.ImageTarget{
				{
					Name: "localhost:5000/test",
				},
			},
			Env: []corev1.EnvVar{
				{
					Name:  "GITHUB_REPO",
					Value: "build-test",
				},
			},
		},
	}
}

func buildAssets() {
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		iferr(exec.Command("docker", "build", "-t", "localhost:5000/controller", "..").Run())
		iferr(exec.Command("docker", "push", "localhost:5000/controller").Run())
		wg.Done()
	}()
	wg.Add(1)
	go func() {
		iferr(exec.Command("docker", "build", "-t", "localhost:5000/actor-base", "-f", "../actor/base/Dockerfile", "..").Run())
		iferr(exec.Command("docker", "push", "localhost:5000/actor-base").Run())
		wg.Done()
	}()
	wg.Add(1)
	go func() {
		iferr(exec.Command("docker", "build", "-t", "localhost:5000/actor-registryv2", "-f", "../actor/registryv2/Dockerfile", "..").Run())
		iferr(exec.Command("docker", "push", "localhost:5000/actor-registryv2").Run())
		wg.Done()
	}()
	wg.Add(1)
	go func() {
		iferr(exec.Command("docker", "build", "-t", "localhost:5000/actor-github", "-f", "../actor/github/Dockerfile", "..").Run())
		iferr(exec.Command("docker", "push", "localhost:5000/actor-github").Run())
		wg.Done()
	}()
	wg.Wait()
}

func newSecret(name string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "oci-image-operator-system",
		},
		StringData: map[string]string{
			"username": "test",
			"password": "test",
		},
	}
}

func newImageFlowTemplate(name string) *buildv1beta1.ImageFlowTemplate {
	return &buildv1beta1.ImageFlowTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		Spec: buildv1beta1.ImageFlowTemplateSpec{
			BaseImage: "localhost:5000/actor-base",
			Detect: buildv1beta1.ImageFlowTemplateSpecTemplate{
				Actor: (*buildv1beta1.ContainerApplyConfiguration)(corev1apply.Container().
					WithName("main").
					WithImage("localhost:5000/actor-github").
					WithEnv(
						corev1apply.EnvVar().WithName("GITHUB_ORG").WithValue("takutakahashi"),
						corev1apply.EnvVar().WithName("GITHUB_TOKEN").WithValue(os.Getenv("GITHUB_TOKEN")),
					),
				),
			},
			Check: buildv1beta1.ImageFlowTemplateSpecTemplate{
				Actor: (*buildv1beta1.ContainerApplyConfiguration)(corev1apply.Container().
					WithName("main").
					WithImage("localhost:5000/actor-registryv2"),
				),
			},
			Upload: buildv1beta1.ImageFlowTemplateSpecTemplate{
				Actor: (*buildv1beta1.ContainerApplyConfiguration)(corev1apply.Container().
					WithName("main").
					WithImage("localhost:5000/actor-github").
					WithEnv(
						corev1apply.EnvVar().WithName("GITHUB_WORKFLOW_FILENAME").WithValue("for_e2e.yaml"),
						corev1apply.EnvVar().WithName("GITHUB_ORG").WithValue("takutakahashi"),
						corev1apply.EnvVar().WithName("GITHUB_TOKEN").WithValue(os.Getenv("GITHUB_TOKEN")),
					),
				),
			},
		},
	}
}
