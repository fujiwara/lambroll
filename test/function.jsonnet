local prefix1_tfstate = std.native('prefix1_tfstate');
local tfstate = std.native('tfstate');
local must_env = std.native('must_env');
local caller = std.native('caller_identity')();

// Test variables for ext_str and ext_code support (issue #507)
local description = std.extVar("description");
local architecture = std.extVar("architecture");
local memory_size = std.extVar("memory_size");
local storage_size = std.extVar("storage_size");
local timeout = std.extVar("timeout");

{
  Architectures: [
    architecture,
  ],
  Description: description,
  EphemeralStorage: {
    Size: storage_size,
  },
  Environment: {
    Variables: {
      JSON: '{{ env `JSON` | json_escape }}',
      PREFIXED_TFSTATE_1: prefix1_tfstate('data.aws_iam_role.lambda.arn'),
      PREFIXED_TFSTATE_2: '{{ prefix2_tfstate `data.aws_iam_role.lambda.arn` }}',
    },
  },
  FunctionName: must_env('FUNCTION_NAME'),
  FileSystemConfigs: [
    {
      Arn: 'arn:aws:elasticfilesystem:ap-northeast-1:%s:access-point/fsap-04fc0858274e7dd9a' % caller.Account,
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
  MemorySize: memory_size,
  Role: tfstate('data.aws_iam_role.lambda.arn'),
  Runtime: 'nodejs16.x',
  Timeout: timeout,
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
