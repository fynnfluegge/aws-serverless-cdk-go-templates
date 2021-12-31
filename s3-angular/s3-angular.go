package main

import (
	"os"

	"github.com/aws/aws-cdk-go/awscdk"
	"github.com/aws/aws-cdk-go/awscdk/awscertificatemanager"
	"github.com/aws/aws-cdk-go/awscdk/awscloudfront"
	"github.com/aws/aws-cdk-go/awscdk/awsiam"
	"github.com/aws/aws-cdk-go/awscdk/awsroute53"
	"github.com/aws/aws-cdk-go/awscdk/awsroute53targets"
	"github.com/aws/aws-cdk-go/awscdk/awss3"
	"github.com/aws/aws-cdk-go/awscdk/awss3assets"
	"github.com/aws/aws-cdk-go/awscdk/awss3deployment"
	"github.com/aws/constructs-go/constructs/v3"
	"github.com/aws/jsii-runtime-go"
)

type S3AngularStackProps struct {
	awscdk.StackProps
	subDomain  string
	domainName string
}

func NewS3AngularStack(scope constructs.Construct, id string, props *S3AngularStackProps) awscdk.Stack {
	var sprops awscdk.StackProps
	if props != nil {
		sprops = props.StackProps
	}

	stack := awscdk.NewStack(scope, &id, &sprops)

	domain := props.subDomain + "." + props.domainName

	cloudfrontOAI := awscloudfront.NewOriginAccessIdentity(stack, jsii.String("MyOriginAccessIdentity"), &awscloudfront.OriginAccessIdentityProps{
		Comment: jsii.String("OAI for " + id),
	})

	zone := awsroute53.HostedZone_FromLookup(stack, jsii.String("MyHostedZone"), &awsroute53.HostedZoneProviderProps{
		DomainName: &props.domainName,
	})

	awscdk.NewCfnOutput(stack, jsii.String("HostedZoneId"), &awscdk.CfnOutputProps{
		Value: zone.HostedZoneId(),
	})

	bucket := awss3.NewBucket(stack, jsii.String("MyS3Bucket"), &awss3.BucketProps{
		BucketName:           &props.domainName,
		WebsiteIndexDocument: jsii.String("index.html"),
		WebsiteErrorDocument: jsii.String("error.html"),
		PublicReadAccess:     jsii.Bool(true),
	})

	bucket.AddToResourcePolicy(awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
		Actions:   jsii.Strings("s3:GetObject"),
		Resources: jsii.Strings(*bucket.ArnForObjects(jsii.String("*"))),
		Principals: &[]awsiam.IPrincipal{
			awsiam.NewCanonicalUserPrincipal(cloudfrontOAI.CloudFrontOriginAccessIdentityS3CanonicalUserId()),
		},
	}))

	awscdk.NewCfnOutput(stack, jsii.String("MyBucketName"), &awscdk.CfnOutputProps{
		Value: bucket.BucketDomainName(),
	})

	certificateArn := awscertificatemanager.NewDnsValidatedCertificate(stack, jsii.String("MySiteCertificate"), &awscertificatemanager.DnsValidatedCertificateProps{
		DomainName: &domain,
		HostedZone: zone,
		Region:     jsii.String("us-east-1"), // Cloudfront only checks this region for certificates.
	})

	awscdk.NewCfnOutput(stack, jsii.String("Certificate"), &awscdk.CfnOutputProps{
		Value: certificateArn.CertificateArn(),
	})

	viewerCertificate := awscloudfront.ViewerCertificate_FromAcmCertificate(
		certificateArn,
		&awscloudfront.ViewerCertificateOptions{
			SslMethod:      awscloudfront.SSLMethod_SNI,
			SecurityPolicy: awscloudfront.SecurityPolicyProtocol_TLS_V1_1_2016,
			Aliases:        jsii.Strings(domain),
		},
	)

	distribution := awscloudfront.NewCloudFrontWebDistribution(stack, jsii.String("MyCloudFrontDistribution"), &awscloudfront.CloudFrontWebDistributionProps{
		ViewerCertificate: viewerCertificate,
		OriginConfigs: &[]*awscloudfront.SourceConfiguration{
			&awscloudfront.SourceConfiguration{
				S3OriginSource: &awscloudfront.S3OriginConfig{
					S3BucketSource:       bucket,
					OriginAccessIdentity: cloudfrontOAI,
				},
				Behaviors: &[]*awscloudfront.Behavior{
					&awscloudfront.Behavior{
						IsDefaultBehavior: jsii.Bool(true),
						Compress:          jsii.Bool(true),
						AllowedMethods:    awscloudfront.CloudFrontAllowedMethods_GET_HEAD_OPTIONS,
					},
				},
			},
		},
	})

	awscdk.NewCfnOutput(stack, jsii.String("CloudFrontWebDistributionId"), &awscdk.CfnOutputProps{
		Value: distribution.DistributionId(),
	})

	awsroute53.NewARecord(stack, jsii.String("MySiteAliasRecord"), &awsroute53.ARecordProps{
		RecordName: &domain,
		Target:     awsroute53.AddressRecordTarget_FromAlias(awsroute53targets.NewCloudFrontTarget(distribution)),
		Zone:       zone,
	})

	deployment := awss3deployment.NewBucketDeployment(stack, jsii.String("MyS3BucketDeployment"), &awss3deployment.BucketDeploymentProps{
		Sources: &[]awss3deployment.ISource{
			awss3deployment.Source_Asset(jsii.String("./simple-angular-app/dist"), &awss3assets.AssetOptions{}),
		},
		DestinationBucket: bucket,
		Distribution:      distribution,
		DistributionPaths: &[]*string{jsii.String("/*")},
	})

	awscdk.NewCfnOutput(stack, jsii.String("Mys3BucketDeployment"), &awscdk.CfnOutputProps{
		Value: deployment.Node().Id(),
	})

	return stack
}

func main() {
	app := awscdk.NewApp(nil)

	NewS3AngularStack(app, "S3AngularStack", &S3AngularStackProps{
		awscdk.StackProps{
			Env: env(),
		},
		app.Node().TryGetContext(jsii.String("subDomain")).(string),
		app.Node().TryGetContext(jsii.String("domainName")).(string),
	})

	app.Synth(nil)
}

func env() *awscdk.Environment {
	return &awscdk.Environment{
		Account: jsii.String(os.Getenv("AWS_ACCOUNT")),
		Region:  jsii.String(os.Getenv("AWS_REGION")),
	}
}
