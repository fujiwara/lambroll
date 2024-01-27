{
  Architectures: [
    'x86_64',
  ],
  Description: std.extVar('Description'),
  EphemeralStorage: {
    Size: 1024,
  },
  Environment: {
    Variables: {
      JSON: '{{ env `JSON` | json_escape }}',
      PREFIXED_TFSTATE_1: '{{ prefix1_tfstate `data.aws_iam_role.lambda.arn` }}',
      PREFIXED_TFSTATE_2: '{{ prefix2_tfstate `data.aws_iam_role.lambda.arn` }}',
    },
  },
  FunctionName: '{{ must_env `FUNCTION_NAME` }}',
  FileSystemConfigs: [
    {
      Arn: 'arn:aws:elasticfilesystem:ap-northeast-1:123456789012:access-point/fsap-04fc0858274e7dd9a',
      LocalMountPath: '/mnt/lambda',
    },
  ],
  Handler: 'index.js',
  LoggingConfig: {
    ApplicationLogLevel: 'DEBUG',
    LogFormat: 'JSON',
    LogGroup: '/aws/lambda/{{ must_env `FUNCTION_NAME` }}/json',
    SystemLogLevel: 'INFO',
  },
  MemorySize: std.extVar('MemorySize'),
  Role: '{{ tfstate `data.aws_iam_role.lambda.arn` }}',
  Runtime: 'nodejs12.x',
  Timeout: 5,
  TracingConfig: {
    Mode: 'PassThrough',
  },
  VpcConfig: {
    SubnetIds: [
      'subnet-08dc9a51660120991',
      'subnet-023e96b860485e2ad',
      'subnet-045cd24ab8e92a20d',
    ],
    SecurityGroupIds: [
      "{{ tfstatef `aws_security_group.internal['%s'].id` (must_env `WORLD`) }}",
    ],
  },
}
