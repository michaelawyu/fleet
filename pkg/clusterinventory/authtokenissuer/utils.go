/*
Copyright (c) Microsoft Corporation.
Licensed under the MIT license.
*/

package authtokenissuer

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/klog/v2"
)

// shouldRecreateTokenSecret checks if a token secret is of the expected type (service account token)
// and is linked with the expected service account.
//
// Token secret needs to be re-created if:
// a) it is not of the service account token type; or
// b) it is not linked with the expected service account; or
// c) it is linked with a service account of the same name but a different UID.
func shouldRecreateTokenSecret(secret *corev1.Secret, svcAccount *corev1.ServiceAccount) bool {
	if secret.Type != corev1.SecretTypeServiceAccountToken {
		klog.V(2).InfoS("The secret is not of the service account token type", "Secret", klog.KObj(secret))
		return true
	}

	svcAccountName, svcAccountRefOK := secret.Annotations[corev1.ServiceAccountNameKey]
	svcAccountUID, svcAccountUIDOK := secret.Annotations[corev1.ServiceAccountUIDKey]
	klog.V(4).InfoS("Checking if a token secret is linked with a service account",
		"Secret", klog.KObj(secret), "ServiceAccount", klog.KObj(svcAccount),
		"IsSvcAccountRefAnnotationFound", svcAccountRefOK, "IsSvcAccountUIDAnnotationFound", svcAccountUIDOK,
		"SvcAccountRefName", svcAccountName, "SvcAccountRefUID", svcAccountUID,
		"SvcAccountName", svcAccount.Name, "SvcAccountUID", string(svcAccount.UID))

	if !svcAccountRefOK || svcAccountName != svcAccount.Name {
		klog.V(2).InfoS("The secret is not linked with a service account", "Secret", klog.KObj(secret), "ServiceAccount", klog.KObj(svcAccount))
		return true
	}
	if svcAccountUIDOK && svcAccountUID != string(svcAccount.UID) {
		klog.V(2).InfoS("The secret is linked with a service account of the same name but a different UID", "Secret", klog.KObj(secret), "ServiceAccount", klog.KObj(svcAccount))
		return true
	}
	return false
}

// isTokenSecretReadyToUse checks if a token secret is ready to use.
//
// A token secret is ready to use if:
// a) it is linked with the expected service account (the UID ref matches with the UID); and
// b) it contains the token and the CA certificate.
func isTokenSecretReadyToUse(secret *corev1.Secret, svcAccount *corev1.ServiceAccount) bool {
	svcAccountUID, svcAccountUIDOK := secret.Annotations[corev1.ServiceAccountUIDKey]
	_, tokenOK := secret.Data[corev1.ServiceAccountTokenKey]
	_, caOK := secret.Data[corev1.ServiceAccountRootCAKey]
	return tokenOK && caOK && (svcAccountUIDOK && svcAccountUID == string(svcAccount.UID))
}

func buildKubeConfigYAML(clusterName, serverHost string, tokenSecret *corev1.Secret) ([]byte, error) {
	clusters := make(map[string]*clientcmdapi.Cluster)
	clusters[clusterName] = &clientcmdapi.Cluster{
		Server: serverHost,
		//InsecureSkipTLSVerify: true,
		CertificateAuthorityData: tokenSecret.Data["ca.crt"],
	}

	contexts := make(map[string]*clientcmdapi.Context)
	contexts[clusterName] = &clientcmdapi.Context{
		Cluster:  clusterName,
		AuthInfo: clusterName,
	}

	authInfos := make(map[string]*clientcmdapi.AuthInfo)
	authInfos[clusterName] = &clientcmdapi.AuthInfo{
		Token: string(tokenSecret.Data["token"]),
	}

	kubeConfig := clientcmdapi.Config{
		Kind:           "Config",
		APIVersion:     "v1",
		Clusters:       clusters,
		Contexts:       contexts,
		AuthInfos:      authInfos,
		CurrentContext: clusterName,
	}
	return clientcmd.Write(kubeConfig)
}
