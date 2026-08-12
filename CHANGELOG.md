# Changelog

## [v1.5.2](https://github.com/fujiwara/lambroll/compare/v1.5.1...v1.5.2) - 2026-08-12

- Bump google.golang.org/grpc from 1.80.0 to 1.82.1 by @dependabot[bot] in https://github.com/fujiwara/lambroll/pull/632
- Add cfn_output and cfn_export template/Jsonnet functions by @fujiwara in https://github.com/fujiwara/lambroll/pull/642

## [v1.5.1](https://github.com/fujiwara/lambroll/compare/v1.5.0...v1.5.1) - 2026-07-17

- Bump golang.org/x/net from 0.52.0 to 0.55.0 by @dependabot[bot] in https://github.com/fujiwara/lambroll/pull/625
- Bump golang.org/x/crypto from 0.51.0 to 0.52.0 by @dependabot[bot] in https://github.com/fujiwara/lambroll/pull/627
- Support S3ObjectStorageMode for self-managed code storage by @fujiwara in https://github.com/fujiwara/lambroll/pull/628
- Bump dependencies (consolidated dependabot PRs) by @fujiwara in https://github.com/fujiwara/lambroll/pull/629
- Fix spurious diff for functions using self-managed code storage by @fujiwara in https://github.com/fujiwara/lambroll/pull/631

## [v1.5.0](https://github.com/fujiwara/lambroll/compare/v1.4.4...v1.5.0) - 2026-06-13

- Exclude GCS and AzureRM tfstate backends from release binaries by @fujiwara in https://github.com/fujiwara/lambroll/pull/598
- fix: sort VPC subnet/security group IDs in diff by @draftcode in https://github.com/fujiwara/lambroll/pull/600
- Add diff --external and unify diff with jsondiff by @fujiwara in https://github.com/fujiwara/lambroll/pull/601
- Replace log.Printf with slog in cli.go by @fujiwara in https://github.com/fujiwara/lambroll/pull/602
- Update AWS SDK and replace deprecated EndpointResolver by @fujiwara in https://github.com/fujiwara/lambroll/pull/603
- ci: replace LocalStack with floci by @fujiwara in https://github.com/fujiwara/lambroll/pull/604
- Fix minor robustness issues and cleanups by @fujiwara in https://github.com/fujiwara/lambroll/pull/605
- Bump dependencies (combined dependabot updates) by @fujiwara in https://github.com/fujiwara/lambroll/pull/612
- Support subcommand-specific flags in option file via unified CLIOptions by @fujiwara in https://github.com/fujiwara/lambroll/pull/614
- Add --mask to hide sensitive values in diff and render output by @fujiwara in https://github.com/fujiwara/lambroll/pull/615
- Unified option file and diff/render --mask (formerly pre-v2) by @fujiwara in https://github.com/fujiwara/lambroll/pull/616
- Fix diff/render of empty environment variables and mask warning by @fujiwara in https://github.com/fujiwara/lambroll/pull/617

## [v1.4.4](https://github.com/fujiwara/lambroll/compare/v1.4.3...v1.4.4) - 2026-05-26
- Add DurableConfig to diff/update handling by @butadora3 in https://github.com/fujiwara/lambroll/pull/594
- Bundle dependabot updates by @fujiwara in https://github.com/fujiwara/lambroll/pull/596

## [v1.4.3](https://github.com/fujiwara/lambroll/compare/v1.4.2...v1.4.3) - 2026-04-17
- Fix ext_str/ext_code from option file being overridden by CLI args by @jirtosterone in https://github.com/fujiwara/lambroll/pull/568
- go fix (modernize) and pin actions. by @fujiwara in https://github.com/fujiwara/lambroll/pull/570
- Bundle dependabot dependency bumps by @fujiwara in https://github.com/fujiwara/lambroll/pull/585
- Bump tablewriter to v1.1.3 by @fujiwara in https://github.com/fujiwara/lambroll/pull/586
- Document FileSystemConfigs support for EFS and S3 Files by @fujiwara in https://github.com/fujiwara/lambroll/pull/587

## [v1.4.2](https://github.com/fujiwara/lambroll/compare/v1.4.1...v1.4.2) - 2026-02-03
- Supports Lambda Managed Instances by @fujiwara in https://github.com/fujiwara/lambroll/pull/554
- add readme for s3 endpoint. by @fujiwara in https://github.com/fujiwara/lambroll/pull/559
- Bump actions/checkout from 5.0.0 to 6.0.1 by @dependabot[bot] in https://github.com/fujiwara/lambroll/pull/555
- Bump actions/setup-go from 6.0.0 to 6.1.0 by @dependabot[bot] in https://github.com/fujiwara/lambroll/pull/549
- Bump github.com/samber/lo from 1.51.0 to 1.52.0 by @dependabot[bot] in https://github.com/fujiwara/lambroll/pull/538
- Bump github.com/itchyny/gojq from 0.12.17 to 0.12.18 by @dependabot[bot] in https://github.com/fujiwara/lambroll/pull/552
- Bump the aws-sdk-go-v2 group across 1 directory with 3 updates by @dependabot[bot] in https://github.com/fujiwara/lambroll/pull/558
- Supports multi tenancy mode. by @fujiwara in https://github.com/fujiwara/lambroll/pull/547
- Bump actions/checkout from 6.0.1 to 6.0.2 by @dependabot[bot] in https://github.com/fujiwara/lambroll/pull/560
- Bump actions/setup-go from 6.1.0 to 6.2.0 by @dependabot[bot] in https://github.com/fujiwara/lambroll/pull/561
- Bump github.com/fujiwara/tfstate-lookup from 1.8.1 to 1.10.0 by @dependabot[bot] in https://github.com/fujiwara/lambroll/pull/566
- Bump golang.org/x/sys from 0.38.0 to 0.40.0 by @dependabot[bot] in https://github.com/fujiwara/lambroll/pull/563
- Bump the aws-sdk-go-v2 group across 1 directory with 2 updates by @dependabot[bot] in https://github.com/fujiwara/lambroll/pull/567

## [v1.4.1](https://github.com/fujiwara/lambroll/compare/v1.4.0...v1.4.1) - 2025-11-21
- Bump golang.org/x/crypto from 0.39.0 to 0.45.0 by @dependabot[bot] in https://github.com/fujiwara/lambroll/pull/543
- supports `aws login` - update to aws-sdk-go-v2 v1.40.0 by @fujiwara in https://github.com/fujiwara/lambroll/pull/545

## [v1.4.0](https://github.com/fujiwara/lambroll/compare/v1.3.2...v1.4.0) - 2025-11-02
- Add args inputs to run lambroll after installation. by @fujiwara in https://github.com/fujiwara/lambroll/pull/527
- Fix invoke command example by @ss49919201 in https://github.com/fujiwara/lambroll/pull/529
- Add comprehensive documentation to README by @fujiwara in https://github.com/fujiwara/lambroll/pull/534
- Add both InvokeFunctionUrl and InvokeFunction permissions for Function URLs by @fujiwara in https://github.com/fujiwara/lambroll/pull/536
- add diff --skip-function option. by @fujiwara in https://github.com/fujiwara/lambroll/pull/539
- Migrate from logutils to sloghandler and refactor logging by @fujiwara in https://github.com/fujiwara/lambroll/pull/540
- Add --log-format option (text|json) by @fujiwara in https://github.com/fujiwara/lambroll/pull/541
- use version.go by @fujiwara in https://github.com/fujiwara/lambroll/pull/542

## [v1.3.2](https://github.com/fujiwara/lambroll/compare/v1.3.1...v1.3.2) - 2025-09-19
- Immutable release by @fujiwara in https://github.com/fujiwara/lambroll/pull/525

## [v1.3.1](https://github.com/fujiwara/lambroll/compare/v1.3.0...v1.3.1) - 2025-08-16
- Fix ext_str and ext_code support in option files by @jirtosterone in https://github.com/fujiwara/lambroll/pull/508
- Add backward compatibility for extstr/extcode fields by @fujiwara in https://github.com/fujiwara/lambroll/pull/514
- Add branch policy to development guidelines by @fujiwara in https://github.com/fujiwara/lambroll/pull/515
- Bump github.com/fujiwara/tfstate-lookup from 1.4.2 to 1.7.0 by @dependabot[bot] in https://github.com/fujiwara/lambroll/pull/520
- Bump github.com/alecthomas/kong from 1.10.0 to 1.12.1 by @dependabot[bot] in https://github.com/fujiwara/lambroll/pull/519
- Bump github.com/google/go-jsonnet from 0.20.0 to 0.21.0 by @dependabot[bot] in https://github.com/fujiwara/lambroll/pull/511
- Bump the aws-sdk-go-v2 group across 1 directory with 5 updates by @dependabot[bot] in https://github.com/fujiwara/lambroll/pull/512
- Bump github.com/samber/lo from 1.49.1 to 1.51.0 by @dependabot[bot] in https://github.com/fujiwara/lambroll/pull/509
- Bump github.com/google/go-cmp from 0.6.0 to 0.7.0 by @dependabot[bot] in https://github.com/fujiwara/lambroll/pull/505

## [v1.3.0](https://github.com/fujiwara/lambroll/compare/v1.2.2...v1.3.0) - 2025-04-22
- Bump golang.org/x/net from 0.36.0 to 0.38.0 by @dependabot in https://github.com/fujiwara/lambroll/pull/496
- Bump github.com/golang-jwt/jwt/v4 from 4.5.1 to 4.5.2 by @dependabot in https://github.com/fujiwara/lambroll/pull/493
- Bump github.com/golang-jwt/jwt/v5 from 5.2.1 to 5.2.2 by @dependabot in https://github.com/fujiwara/lambroll/pull/492
- Bump github.com/aereal/jsondiff from 0.4.0 to 0.4.1 by @dependabot in https://github.com/fujiwara/lambroll/pull/481
- Bump golang.org/x/sys from 0.30.0 to 0.31.0 by @dependabot in https://github.com/fujiwara/lambroll/pull/494
- Bump the aws-sdk-go-v2 group across 1 directory with 5 updates by @dependabot in https://github.com/fujiwara/lambroll/pull/484
- Bump github.com/alecthomas/kong from 1.6.1 to 1.10.0 by @dependabot in https://github.com/fujiwara/lambroll/pull/495
- Bump github.com/samber/lo from 1.47.0 to 1.49.1 by @dependabot in https://github.com/fujiwara/lambroll/pull/482
- ignore AWS managed tags for tag/untag operation. by @fujiwara in https://github.com/fujiwara/lambroll/pull/500
- add --skip-configuration to deploy command. by @fujiwara in https://github.com/fujiwara/lambroll/pull/499

## [v1.2.2](https://github.com/fujiwara/lambroll/compare/v1.2.1...v1.2.2) - 2025-03-19
- Bump golang.org/x/net from 0.34.0 to 0.36.0 by @dependabot in https://github.com/fujiwara/lambroll/pull/488
- Fix download function to use context for HTTP requests by @fujiwara in https://github.com/fujiwara/lambroll/pull/489
- Fixed error handling for --keep-versions by @mashiike in https://github.com/fujiwara/lambroll/pull/491

## [v1.2.1](https://github.com/fujiwara/lambroll/compare/v1.2.0...v1.2.1) - 2025-03-12
- Add diff --exit-code by @fujiwara in https://github.com/fujiwara/lambroll/pull/486

## [v1.2.0](https://github.com/fujiwara/lambroll/compare/v1.1.3...v1.2.0) - 2025-02-10
- remove type alias by @fujiwara in https://github.com/fujiwara/lambroll/pull/450
- load envfile opts first by @ijin in https://github.com/fujiwara/lambroll/pull/456
- Bump golang.org/x/crypto from 0.24.0 to 0.31.0 by @dependabot in https://github.com/fujiwara/lambroll/pull/455
- feat: Add lambroll version file input by @taxintt in https://github.com/fujiwara/lambroll/pull/457
- update v1-actions-test for version-file by @fujiwara in https://github.com/fujiwara/lambroll/pull/460
- Bump the aws-sdk-go-v2 group across 1 directory with 5 updates by @dependabot in https://github.com/fujiwara/lambroll/pull/459
- Bump github.com/alecthomas/kong from 0.9.0 to 1.6.1 by @dependabot in https://github.com/fujiwara/lambroll/pull/461
- Bump github.com/golang-jwt/jwt/v4 from 4.5.0 to 4.5.1 by @dependabot in https://github.com/fujiwara/lambroll/pull/448
- Bump github.com/fatih/color from 1.17.0 to 1.18.0 by @dependabot in https://github.com/fujiwara/lambroll/pull/445
- update modules: golang.org/x/net v0.34.0 by @fujiwara in https://github.com/fujiwara/lambroll/pull/462
- Add Songmu/tagpr for release management. by @fujiwara in https://github.com/fujiwara/lambroll/pull/463
- Add layer_arn function. by @fujiwara in https://github.com/fujiwara/lambroll/pull/442
- Add --option flag to read default options. by @fujiwara in https://github.com/fujiwara/lambroll/pull/451
- fix: use the del() function to ignore a given path by @aereal in https://github.com/fujiwara/lambroll/pull/467
- Bump github.com/hashicorp/go-slug from 0.15.0 to 0.16.3 by @dependabot in https://github.com/fujiwara/lambroll/pull/468
- Make the specification clear for overriding flag priorities. by @fujiwara in https://github.com/fujiwara/lambroll/pull/469
- Add hosted arm runner. by @fujiwara in https://github.com/fujiwara/lambroll/pull/470
- Add init --unzip by @fujiwara in https://github.com/fujiwara/lambroll/pull/472
- Bump github.com/aereal/jsondiff from 0.3.0 to 0.4.0 by @dependabot in https://github.com/fujiwara/lambroll/pull/474
- Bump github.com/fujiwara/ssm-lookup from 0.1.0 to 0.1.1 by @dependabot in https://github.com/fujiwara/lambroll/pull/476

## [v1.1.3](https://github.com/fujiwara/lambroll/compare/v1.1.2...v1.1.3) - 2024-10-30
- fix: lambroll diff to ignore unchanged VpcConfig.Ipv6AllowedForDualStack values by @mackee in https://github.com/fujiwara/lambroll/pull/443

## [v1.1.2](https://github.com/fujiwara/lambroll/compare/v1.1.0...v1.1.2) - 2024-10-02
- Bump github.com/fujiwara/tfstate-lookup from 1.3.2 to 1.4.2 by @dependabot in https://github.com/fujiwara/lambroll/pull/439
- Bump golang.org/x/sys from 0.24.0 to 0.25.0 by @dependabot in https://github.com/fujiwara/lambroll/pull/438
- Bump the aws-sdk-go-v2 group with 5 updates by @dependabot in https://github.com/fujiwara/lambroll/pull/437
- fix: Correct color configuration in CLI by @soh335 in https://github.com/fujiwara/lambroll/pull/436
- --no-color also works. by @fujiwara in https://github.com/fujiwara/lambroll/pull/441

## [v1.1.1](https://github.com/fujiwara/lambroll/compare/v0.14.6...v1.1.1) - 2023-11-21
- support LoggingConfig by @fujiwara in https://github.com/fujiwara/lambroll/pull/334

## [v1.1.0](https://github.com/fujiwara/lambroll/compare/v1.0.6...v1.1.0) - 2024-09-02
- Add Jsonnet native functions. by @fujiwara in https://github.com/fujiwara/lambroll/pull/429
- Add --symlink to archive symlinks as is. by @fujiwara in https://github.com/fujiwara/lambroll/pull/430
- Bump the aws-sdk-go-v2 group with 5 updates by @dependabot in https://github.com/fujiwara/lambroll/pull/431
- Bump github.com/fatih/color from 1.16.0 to 1.17.0 by @dependabot in https://github.com/fujiwara/lambroll/pull/435
- Bump github.com/samber/lo from 1.44.0 to 1.47.0 by @dependabot in https://github.com/fujiwara/lambroll/pull/434
- Bump github.com/go-test/deep from 1.1.0 to 1.1.1 by @dependabot in https://github.com/fujiwara/lambroll/pull/433
- Bump golang.org/x/sys from 0.22.0 to 0.24.0 by @dependabot in https://github.com/fujiwara/lambroll/pull/432

## [v1.0.6](https://github.com/fujiwara/lambroll/compare/v1.0.5...v1.0.6) - 2024-08-09
- update readme: deploy via s3 by @fujiwara in https://github.com/fujiwara/lambroll/pull/421
- treat symlink as file when adding to zip / #402 by @fujiwara in https://github.com/fujiwara/lambroll/pull/422
- chore: treat symlink as file when adding to zip by @j3tm0t0 in https://github.com/fujiwara/lambroll/pull/402
- Bump the aws-sdk-go-v2 group across 1 directory with 5 updates by @dependabot in https://github.com/fujiwara/lambroll/pull/424
- Bump github.com/shogo82148/go-retry from 1.1.1 to 1.3.1 by @dependabot in https://github.com/fujiwara/lambroll/pull/423
- Bump github.com/fujiwara/ssm-lookup from 0.0.1 to 0.1.0 by @dependabot in https://github.com/fujiwara/lambroll/pull/419
- update go-jq and kong to latest by @fujiwara in https://github.com/fujiwara/lambroll/pull/425
- feat(action.yml): auto detect os and arch by @bungoume in https://github.com/fujiwara/lambroll/pull/417
- V1 actions test by @fujiwara in https://github.com/fujiwara/lambroll/pull/426
- Fix/actions bash3 by @fujiwara in https://github.com/fujiwara/lambroll/pull/427

## [v1.0.5](https://github.com/fujiwara/lambroll/compare/v1.0.4...v1.0.5) - 2024-07-10
- Bump golang.org/x/net from 0.17.0 to 0.23.0 by @dependabot in https://github.com/fujiwara/lambroll/pull/395
- set empty list to Layers explicitly by @fujiwara in https://github.com/fujiwara/lambroll/pull/404
- fix fillDefaultValues by @fujiwara in https://github.com/fujiwara/lambroll/pull/405
- fix: default LoggingConfig by @fujiwara in https://github.com/fujiwara/lambroll/pull/414
- fix: for goreleaser v2 by @fujiwara in https://github.com/fujiwara/lambroll/pull/413
- feat(action.yml): add os and arch options by @rinx in https://github.com/fujiwara/lambroll/pull/412
- Bump goreleaser/goreleaser-action from 5 to 6 by @dependabot in https://github.com/fujiwara/lambroll/pull/411
- Bump github.com/hashicorp/go-retryablehttp from 0.7.1 to 0.7.7 by @dependabot in https://github.com/fujiwara/lambroll/pull/406
- Bump github.com/Azure/azure-sdk-for-go/sdk/azidentity from 1.3.1 to 1.6.0 by @dependabot in https://github.com/fujiwara/lambroll/pull/401
- Bump golang.org/x/sys from 0.20.0 to 0.22.0 by @dependabot in https://github.com/fujiwara/lambroll/pull/415
- update to google.golang.org/grpc@v1.56.3 by @fujiwara in https://github.com/fujiwara/lambroll/pull/416
- Bump the aws-sdk-go-v2 group across 1 directory with 4 updates by @dependabot in https://github.com/fujiwara/lambroll/pull/410
- Bump github.com/samber/lo from 1.38.1 to 1.44.0 by @dependabot in https://github.com/fujiwara/lambroll/pull/407
- Bump github.com/google/go-cmp from 0.5.9 to 0.6.0 by @dependabot in https://github.com/fujiwara/lambroll/pull/372

## [v1.0.4](https://github.com/fujiwara/lambroll/compare/v1.0.3...v1.0.4) - 2024-04-19
- fix spelling. by @fujiwara in https://github.com/fujiwara/lambroll/pull/391
- update tfstate-lookup to v1.1.6 by @fujiwara in https://github.com/fujiwara/lambroll/pull/393
- Fix: `--ignore .ImageConfig` didn't work. by @fujiwara in https://github.com/fujiwara/lambroll/pull/394
- Support CloudFront OAC by @fujiwara in https://github.com/fujiwara/lambroll/pull/392

## [v1.0.3](https://github.com/fujiwara/lambroll/compare/v1.0.2...v1.0.3) - 2024-04-05
- no need to check the aliased version before deleting. by @fujiwara in https://github.com/fujiwara/lambroll/pull/390
- Bump github.com/kayac/go-config from 0.6.0 to 0.7.0 by @dependabot in https://github.com/fujiwara/lambroll/pull/375

## [v1.0.2](https://github.com/fujiwara/lambroll/compare/v1.0.1...v1.0.2) - 2024-03-29
- fix typo by @fujiwara in https://github.com/fujiwara/lambroll/pull/359
- add --payload flag to invoke command. by @fujiwara in https://github.com/fujiwara/lambroll/pull/362
- fix jsondiff version to released. by @fujiwara in https://github.com/fujiwara/lambroll/pull/363
- version fix docs and samples by @yjszk in https://github.com/fujiwara/lambroll/pull/367
- Bump golang.org/x/net from 0.14.0 to 0.17.0 by @dependabot in https://github.com/fujiwara/lambroll/pull/365
- Bump google.golang.org/protobuf from 1.26.0 to 1.33.0 by @dependabot in https://github.com/fujiwara/lambroll/pull/378
- Bump golang.org/x/crypto from 0.12.0 to 0.17.0 by @dependabot in https://github.com/fujiwara/lambroll/pull/364
- add init --force-overwrite flag. by @fujiwara in https://github.com/fujiwara/lambroll/pull/377
- Support to `rollback --alias`. by @fujiwara in https://github.com/fujiwara/lambroll/pull/366

## [v1.0.1](https://github.com/fujiwara/lambroll/compare/v1.0.0...v1.0.1) - 2024-02-14
- Fix options for compatibility with previous versions by @miztch in https://github.com/fujiwara/lambroll/pull/358

## [v1.0.0](https://github.com/fujiwara/lambroll/compare/v0.14.4...v1.0.0) - 2024-02-08
- Ignore on deploy by @fujiwara in https://github.com/fujiwara/lambroll/pull/283
- aws-sdk-go-v2 by @fujiwara in https://github.com/fujiwara/lambroll/pull/306
- use Kong instead of kingpin. by @fujiwara in https://github.com/fujiwara/lambroll/pull/315
- update tfstate-lookup v1.1.4 by @fujiwara in https://github.com/fujiwara/lambroll/pull/316
- docs: add the installation guide with aqua by @suzuki-shunsuke in https://github.com/fujiwara/lambroll/pull/318
- dependabot for actions by @fujiwara in https://github.com/fujiwara/lambroll/pull/304
- Ssm plugin by @fujiwara in https://github.com/fujiwara/lambroll/pull/319
- Add render subcommand by @fujiwara in https://github.com/fujiwara/lambroll/pull/326
- V1/archive by @fujiwara in https://github.com/fujiwara/lambroll/pull/327
- fix: added zero value check for ImageConfigRespose by @ponkio-o in https://github.com/fujiwara/lambroll/pull/329
- add diff -u by default by @fujiwara in https://github.com/fujiwara/lambroll/pull/328
- Support LoggingConfig (for v1) by @fujiwara in https://github.com/fujiwara/lambroll/pull/333
- Add status command by @fujiwara in https://github.com/fujiwara/lambroll/pull/336
- add deploy --function-url feature by @fujiwara in https://github.com/fujiwara/lambroll/pull/330
- lambroll status should be simple by @fujiwara in https://github.com/fujiwara/lambroll/pull/344
- accept LAMBROLL_* environment variables as flag valueis. by @fujiwara in https://github.com/fujiwara/lambroll/pull/345
- switch to goreleaser by @fujiwara in https://github.com/fujiwara/lambroll/pull/346
- fix segfault when creating a new function. by @fujiwara in https://github.com/fujiwara/lambroll/pull/347
- lambroll diff works even if the lambda function is not found. by @fujiwara in https://github.com/fujiwara/lambroll/pull/348
- Fix/status by @fujiwara in https://github.com/fujiwara/lambroll/pull/349
- Add --ignore flag to deploy and diff command. by @fujiwara in https://github.com/fujiwara/lambroll/pull/281
- Fix/ignore env by @fujiwara in https://github.com/fujiwara/lambroll/pull/350
- fix failed to diff when a remote function is not found. by @fujiwara in https://github.com/fujiwara/lambroll/pull/351
- Fix/trace log by @fujiwara in https://github.com/fujiwara/lambroll/pull/352
- init --function-url creates a default file by @fujiwara in https://github.com/fujiwara/lambroll/pull/353
- Skip checking to update PackageType when the new function is created. by @fujiwara in https://github.com/fujiwara/lambroll/pull/354
- fix diff output of function url. by @fujiwara in https://github.com/fujiwara/lambroll/pull/355
- fix: deploy function url after create function. by @fujiwara in https://github.com/fujiwara/lambroll/pull/356

## [v0.14.7](https://github.com/fujiwara/lambroll/compare/v0.14.6...v0.14.7) - 2023-11-21
- support LoggingConfig by @fujiwara in https://github.com/fujiwara/lambroll/pull/334

## [v0.14.6](https://github.com/fujiwara/lambroll/compare/v0.14.5...v0.14.6) - 2023-10-19
- docs: add the installation guide with aqua by @suzuki-shunsuke in https://github.com/fujiwara/lambroll/pull/318
- dependabot for actions by @fujiwara in https://github.com/fujiwara/lambroll/pull/304
- fix: added zero value check for ImageConfigRespose by @ponkio-o in https://github.com/fujiwara/lambroll/pull/329

## [v0.14.5](https://github.com/fujiwara/lambroll/compare/v0.14.4...v0.14.5) - 2023-10-06
- update tfstate-lookup v1.1.4 by @fujiwara in https://github.com/fujiwara/lambroll/pull/316

## [v0.14.4](https://github.com/fujiwara/lambroll/compare/v0.14.3...v0.14.4) - 2023-09-22
- Add runtime to output of versions command. by @fujiwara in https://github.com/fujiwara/lambroll/pull/309
- Wait until the function code has been successfully updated by @minamijoyo in https://github.com/fujiwara/lambroll/pull/314

## [v0.14.3](https://github.com/fujiwara/lambroll/compare/v0.14.2...v0.14.3) - 2023-08-10
- Reduce diff by @fujiwara in https://github.com/fujiwara/lambroll/pull/293
- Added an option not listed in the README by @bungoume in https://github.com/fujiwara/lambroll/pull/277
- fix readme by @fujiwara in https://github.com/fujiwara/lambroll/pull/294
- Go 1.20 by @fujiwara in https://github.com/fujiwara/lambroll/pull/296
- Fix readme edge by @fujiwara in https://github.com/fujiwara/lambroll/pull/305
- fill ImageConfig on init by @chaya2z in https://github.com/fujiwara/lambroll/pull/307
- reset ImageConfig explicitly when it is removed. by @fujiwara in https://github.com/fujiwara/lambroll/pull/308

## [v0.14.2](https://github.com/fujiwara/lambroll/compare/v0.14.1...v0.14.2) - 2023-01-23
- fix error handling on UpdateFunctionCode. by @fujiwara in https://github.com/fujiwara/lambroll/pull/289

## [v0.14.1](https://github.com/fujiwara/lambroll/compare/v0.14.0...v0.14.1) - 2022-11-30
- Fix panic on init by @fujiwara in https://github.com/fujiwara/lambroll/pull/276

## [v0.14.0](https://github.com/fujiwara/lambroll/compare/v0.13.0...v0.14.0) - 2022-11-29
- Support to SnapStart. by @fujiwara in https://github.com/fujiwara/lambroll/pull/273

## [v0.13.0](https://github.com/fujiwara/lambroll/compare/v0.12.9...v0.13.0) - 2022-06-10
- Support multiple tfstate files by @Liooo in https://github.com/fujiwara/lambroll/pull/236
- fill default ephemeral storage size to 512 by @fujiwara in https://github.com/fujiwara/lambroll/pull/242
- Fill default values to CreateFunctionInput before deploy. by @fujiwara in https://github.com/fujiwara/lambroll/pull/243
- implements for diff --code by @fujiwara in https://github.com/fujiwara/lambroll/pull/232

## [v0.12.9](https://github.com/fujiwara/lambroll/compare/v0.12.8...v0.12.9) - 2022-04-14
- Bump github.com/aws/aws-sdk-go from 1.40.52 to 1.43.37 by @dependabot in https://github.com/fujiwara/lambroll/pull/226
- Supports EphemeralStorage by @fujiwara in https://github.com/fujiwara/lambroll/pull/227
- set default --function variable by @fujiwara in https://github.com/fujiwara/lambroll/pull/228
