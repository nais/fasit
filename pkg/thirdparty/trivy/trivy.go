package trivy

import (
	"fmt"

	"github.com/aquasecurity/trivy-operator/pkg/apis/aquasecurity/v1alpha1"
	"github.com/nais/fasit/pkg/k8s"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

type Reporter interface {
	GetConfigAuditReports(env string) ([]*v1alpha1.ConfigAuditReport, error)
	GetClusterComplianceReports(env string) ([]*v1alpha1.ClusterComplianceReport, error)
	GetClusterRbacReports(env string) ([]*v1alpha1.RbacAssessmentReport, error)
	RbacAssessmentSummaryTotal(reports []*v1alpha1.RbacAssessmentReport) v1alpha1.RbacAssessmentSummary
	ConfigAuditReportsSummaryTotal(reports []*v1alpha1.ConfigAuditReport) v1alpha1.ConfigAuditSummary
	ClusterComplianceReportsSummaryTotal(reports []*v1alpha1.ClusterComplianceReport) v1alpha1.ComplianceSummary
}

type Client struct {
	client *k8s.Client
	Log    *logrus.Logger
}

func NewReporter(k8sClient *k8s.Client, log *logrus.Logger) *Client {
	return &Client{
		client: k8sClient,
		Log:    log,
	}
}

func (c *Client) GetConfigAuditReports(env string) ([]*v1alpha1.ConfigAuditReport, error) {
	obj, err := c.client.GetUnstructuredConfigAuditReports(env)
	if err != nil {
		return nil, fmt.Errorf("get unstructured config audit reports: %w", err)
	}
	auditReports := make([]*v1alpha1.ConfigAuditReport, 0)
	for _, o := range obj {
		car, err := parseConfigAuditReports(o)
		if err != nil {
			return nil, fmt.Errorf("parse config audit reports: %w", err)
		}
		auditReports = append(auditReports, car)
	}
	return auditReports, nil
}

func parseConfigAuditReports(obj interface{}) (*v1alpha1.ConfigAuditReport, error) {
	if obj == nil {
		return nil, nil
	}
	car := v1alpha1.ConfigAuditReport{}
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.(*unstructured.Unstructured).Object, &car)
	if err != nil {
		return nil, err
	}
	return &car, nil
}

func (c *Client) GetClusterComplianceReports(env string) ([]*v1alpha1.ClusterComplianceReport, error) {
	obj, err := c.client.GetUnstructuredClusterComplianceReports(env)
	if err != nil {
		return nil, fmt.Errorf("get unstructured cluster compliance reports: %w", err)
	}
	clusterComplianceReports := make([]*v1alpha1.ClusterComplianceReport, 0)
	for _, o := range obj {
		ccr, err := parseClusterComplianceReport(o)
		if err != nil {
			return nil, fmt.Errorf("parse cluster compliance reports: %w", err)
		}
		clusterComplianceReports = append(clusterComplianceReports, ccr)
	}
	return clusterComplianceReports, nil
}

func parseClusterComplianceReport(obj interface{}) (*v1alpha1.ClusterComplianceReport, error) {
	if obj == nil {
		return nil, nil
	}
	report := v1alpha1.ClusterComplianceReport{}
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.(*unstructured.Unstructured).Object, &report)
	if err != nil {
		return nil, err
	}
	return &report, nil
}

func (c *Client) GetClusterRbacReports(env string) ([]*v1alpha1.RbacAssessmentReport, error) {
	obj, err := c.client.GetUnstructuredClusterRbacReports(env)
	if err != nil {
		return nil, fmt.Errorf("get unstructured cluster rbac reports: %w", err)
	}
	clusterRbacReports := make([]*v1alpha1.RbacAssessmentReport, 0)
	for _, o := range obj {
		crr, err := parseClusterRbacReport(o)
		if err != nil {
			return nil, fmt.Errorf("parse cluster rbac reports: %w", err)
		}
		clusterRbacReports = append(clusterRbacReports, crr)
	}
	return clusterRbacReports, nil
}

func parseClusterRbacReport(obj interface{}) (*v1alpha1.RbacAssessmentReport, error) {
	if obj == nil {
		return nil, nil
	}
	report := v1alpha1.RbacAssessmentReport{}
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.(*unstructured.Unstructured).Object, &report)
	if err != nil {
		return nil, err
	}
	return &report, nil
}

func (c *Client) RbacAssessmentSummaryTotal(reports []*v1alpha1.RbacAssessmentReport) v1alpha1.RbacAssessmentSummary {
	checks := make([]v1alpha1.Check, 0)
	for _, report := range reports {
		checks = append(checks, report.Report.Checks...)
	}
	return v1alpha1.RbacAssessmentSummaryFromChecks(checks)
}

func (c *Client) ConfigAuditReportsSummaryTotal(reports []*v1alpha1.ConfigAuditReport) v1alpha1.ConfigAuditSummary {
	checks := make([]v1alpha1.Check, 0)
	for _, report := range reports {
		checks = append(checks, report.Report.Checks...)
	}
	return v1alpha1.ConfigAuditSummaryFromChecks(checks)
}

func (c *Client) ClusterComplianceReportsSummaryTotal(reports []*v1alpha1.ClusterComplianceReport) v1alpha1.ComplianceSummary {
	result := v1alpha1.ComplianceSummary{}
	for _, report := range reports {
		result.PassCount += report.Status.Summary.PassCount
		result.FailCount += report.Status.Summary.FailCount
	}
	return result
}
