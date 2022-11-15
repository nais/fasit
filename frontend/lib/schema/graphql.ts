import { gql } from '@apollo/client'
import * as Apollo from '@apollo/client'
export type Maybe<T> = T | null
export type InputMaybe<T> = Maybe<T>
export type Exact<T extends { [key: string]: unknown }> = {
  [K in keyof T]: T[K]
}
export type MakeOptional<T, K extends keyof T> = Omit<T, K> & {
  [SubKey in K]?: Maybe<T[SubKey]>
}
export type MakeMaybe<T, K extends keyof T> = Omit<T, K> & {
  [SubKey in K]: Maybe<T[SubKey]>
}
const defaultOptions = {} as const
/** All built-in and custom scalars, mapped to their actual values */
export type Scalars = {
  ID: string
  String: string
  Boolean: boolean
  Int: number
  Float: number
  Map: any
  RawMessage: any
  Time: any
}

export type AuditLog = {
  __typename?: 'AuditLog'
  actor: Scalars['String']
  createdAt: Scalars['Time']
  description: Scalars['String']
  objectId: Scalars['String']
  objectType: Scalars['String']
}

export enum ConditionStatus {
  False = 'FALSE',
  True = 'TRUE',
  Unknown = 'UNKNOWN',
}

export type ConfigOverride = {
  __typename?: 'ConfigOverride'
  environment: Environment
  keys: Array<Scalars['String']>
}

export enum ConfigType {
  Bool = 'BOOL',
  Int = 'INT',
  String = 'STRING',
  StringArray = 'STRING_ARRAY',
}

export type Configuration = {
  chartValue: Scalars['RawMessage']
  created: Scalars['Time']
  description: Scalars['String']
  displayName: Scalars['String']
  feature: Feature
  id: Scalars['ID']
  key: Scalars['String']
  required: Scalars['Boolean']
  secret: Scalars['Boolean']
  type: ConfigType
  value: Scalars['RawMessage']
}

export type Dependency = {
  __typename?: 'Dependency'
  allOf: Array<Scalars['String']>
  anyOf: Array<Scalars['String']>
}

export type EnvConfig = {
  __typename?: 'EnvConfig'
  configuration: Array<Configuration>
  mapping: Array<MappingValue>
}

export type EnvConfiguration = Configuration & {
  __typename?: 'EnvConfiguration'
  chartValue: Scalars['RawMessage']
  created: Scalars['Time']
  description: Scalars['String']
  displayName: Scalars['String']
  environment: Environment
  feature: Feature
  id: Scalars['ID']
  key: Scalars['String']
  required: Scalars['Boolean']
  secret: Scalars['Boolean']
  type: ConfigType
  value: Scalars['RawMessage']
}

export type Environment = {
  __typename?: 'Environment'
  auditLog: Array<AuditLog>
  created: Scalars['Time']
  description?: Maybe<Scalars['String']>
  featureStates: Array<FeatureState>
  health: Health
  id: Scalars['ID']
  kind: EnvironmentKind
  lastModified: Scalars['Time']
  name: Scalars['String']
  nodes: Array<KubernetesNode>
  releases: Array<Release>
  tenant: Tenant
  values: Array<EnvironmentValue>
  warnings: Array<Warning>
}

export type EnvironmentAuditLogArgs = {
  featureName?: InputMaybe<Scalars['String']>
}

/** EnvironmentCreate contains metadata for creating an environment */
export type EnvironmentCreate = {
  description?: InputMaybe<Scalars['String']>
  kind: EnvironmentKind
  name: Scalars['String']
  tenantID: Scalars['ID']
}

export enum EnvironmentKind {
  Management = 'MANAGEMENT',
  Onprem = 'ONPREM',
  Tenant = 'TENANT',
}

/** UpdateEnvironment contains metadata for updating an environment */
export type EnvironmentUpdate = {
  /** description of the environment */
  description?: InputMaybe<Scalars['String']>
}

export type EnvironmentValue = {
  __typename?: 'EnvironmentValue'
  key: Scalars['String']
  value: Scalars['RawMessage']
}

export type Feature = {
  __typename?: 'Feature'
  chart: Scalars['String']
  config: Scalars['RawMessage']
  configoverrides: Array<ConfigOverride>
  dependsOn: Array<Dependency>
  environmentKinds: Array<EnvironmentKind>
  name: Scalars['String']
  outdatedInfo: Array<Maybe<OutdatedInfo>>
  repo: Scalars['String']
  rolloutSummaries: Array<RolloutSummary>
  source: Scalars['String']
  version: Scalars['String']
}

export type FeatureState = {
  __typename?: 'FeatureState'
  configuration: EnvConfig
  created?: Maybe<Scalars['Time']>
  enabled: Scalars['Boolean']
  feature: Feature
  lastModified?: Maybe<Scalars['Time']>
  missingDependencies: Array<Feature>
  rolloutStatus: RolloutStatus
}

export type FeatureWarning = Warning & {
  __typename?: 'FeatureWarning'
  environment: Environment
  feature: Feature
  message: Scalars['String']
}

export type GlobalConfiguration = Configuration & {
  __typename?: 'GlobalConfiguration'
  chartValue: Scalars['RawMessage']
  created: Scalars['Time']
  description: Scalars['String']
  displayName: Scalars['String']
  feature: Feature
  id: Scalars['ID']
  key: Scalars['String']
  required: Scalars['Boolean']
  secret: Scalars['Boolean']
  type: ConfigType
  value: Scalars['RawMessage']
}

export type Health = {
  __typename?: 'Health'
  reportedAt: Scalars['Time']
}

export type KubernetesNode = {
  __typename?: 'KubernetesNode'
  allocatable: KubernetesNodeResources
  architecture: Scalars['String']
  capacity: KubernetesNodeResources
  conditions: Array<KubernetesNodeCondition>
  containerRuntimeVersion: Scalars['String']
  internalIP: Scalars['String']
  kernelVersion: Scalars['String']
  kubeProxyVersion: Scalars['String']
  kubeletVersion: Scalars['String']
  name: Scalars['String']
  operatingSystem: Scalars['String']
  osImage: Scalars['String']
}

export type KubernetesNodeCondition = {
  __typename?: 'KubernetesNodeCondition'
  lastHeartbeat: Scalars['Time']
  lastTransition: Scalars['Time']
  message: Scalars['String']
  reason: Scalars['String']
  status: ConditionStatus
  type: KubernetesNodeConditionType
}

export enum KubernetesNodeConditionType {
  DiskPressure = 'DISK_PRESSURE',
  MemoryPressure = 'MEMORY_PRESSURE',
  NetworkUnavailable = 'NETWORK_UNAVAILABLE',
  PidPressure = 'PID_PRESSURE',
  Ready = 'READY',
}

export type KubernetesNodeResources = {
  __typename?: 'KubernetesNodeResources'
  cpu: Scalars['Int']
  memory: Scalars['Int']
  pods: Scalars['Int']
  storage: Scalars['Int']
}

export type MappingValue = {
  __typename?: 'MappingValue'
  displayName: Scalars['String']
  key: Scalars['String']
  value: Scalars['RawMessage']
}

export type Mutation = {
  __typename?: 'Mutation'
  configurationCreate: Configuration
  configurationDelete: Scalars['Boolean']
  configurationUpdate: Configuration
  environmentCreate: Environment
  /** updateEnvironment updates an existing environment */
  environmentUpdate: Environment
  featureStateSave: FeatureState
  tenantCreate: Tenant
}

export type MutationConfigurationCreateArgs = {
  configuration: NewConfiguration
}

export type MutationConfigurationDeleteArgs = {
  id: Scalars['ID']
}

export type MutationConfigurationUpdateArgs = {
  configuration: UpdateConfiguration
  id: Scalars['ID']
}

export type MutationEnvironmentCreateArgs = {
  environment: EnvironmentCreate
}

export type MutationEnvironmentUpdateArgs = {
  id: Scalars['ID']
  input: EnvironmentUpdate
}

export type MutationFeatureStateSaveArgs = {
  enabled: Scalars['Boolean']
  envID: Scalars['ID']
  feature: Scalars['String']
}

export type MutationTenantCreateArgs = {
  tenant: TenantCreate
}

export type NaisdWarning = Warning & {
  __typename?: 'NaisdWarning'
  environment: Environment
  message: Scalars['String']
}

export type NewConfiguration = {
  description?: InputMaybe<Scalars['String']>
  environmentID?: InputMaybe<Scalars['ID']>
  feature: Scalars['String']
  key: Scalars['String']
  value: Scalars['RawMessage']
}

export type OutdatedInfo = {
  __typename?: 'OutdatedInfo'
  dependency: Scalars['Boolean']
  dependencyName: Scalars['String']
  feature: Feature
  newVersion: Scalars['String']
}

export type Query = {
  __typename?: 'Query'
  configuration: EnvConfig
  envConfig: EnvConfig
  /** Environment returns the given environment. */
  environment: Environment
  /** EnvironmentByName returns the given environment by tenantName and name. */
  environmentByNames: Environment
  /** Environments returns the environments for a tenant. */
  environments: Array<Environment>
  feature: Feature
  featureState: FeatureState
  featureStatus: Status
  features: Array<Feature>
  helmValues: Scalars['RawMessage']
  outdatedInfo: Array<Maybe<OutdatedInfo>>
  rolloutSummary: RolloutSummary
  /** tenant returns the given tenant. */
  tenant: Tenant
  tenants: Array<Tenant>
  /** userInfo returns the user. */
  userInfo?: Maybe<UserInfo>
  values: Scalars['Map']
}

export type QueryConfigurationArgs = {
  envID?: InputMaybe<Scalars['ID']>
  feature: Scalars['String']
}

export type QueryEnvConfigArgs = {
  envID: Scalars['ID']
  feature: Scalars['String']
}

export type QueryEnvironmentArgs = {
  id: Scalars['ID']
}

export type QueryEnvironmentByNamesArgs = {
  environmentName: Scalars['String']
  tenantName: Scalars['String']
}

export type QueryEnvironmentsArgs = {
  tenantID: Scalars['ID']
}

export type QueryFeatureArgs = {
  name: Scalars['String']
}

export type QueryFeatureStateArgs = {
  envID: Scalars['ID']
  feature: Scalars['String']
}

export type QueryFeatureStatusArgs = {
  envID: Scalars['ID']
  feature: Scalars['String']
}

export type QueryFeaturesArgs = {
  kind?: InputMaybe<EnvironmentKind>
}

export type QueryHelmValuesArgs = {
  envID: Scalars['ID']
  feature: Scalars['String']
}

export type QueryRolloutSummaryArgs = {
  id: Scalars['ID']
}

export type QueryTenantArgs = {
  id?: InputMaybe<Scalars['ID']>
  slug?: InputMaybe<Scalars['String']>
}

export type QueryValuesArgs = {
  env: Scalars['ID']
  feature: Scalars['String']
}

export type Release = {
  __typename?: 'Release'
  created: Scalars['Time']
  feature?: Maybe<Feature>
  lastDeployed: Scalars['Time']
  lastModified: Scalars['Time']
  name: Scalars['String']
  revision: Scalars['Int']
  status: Scalars['String']
  version: Scalars['String']
}

export type Rollout = {
  __typename?: 'Rollout'
  changeset: RolloutChangeset
  created: Scalars['Time']
  environment: Environment
  events: Array<RolloutEvent>
  id: Scalars['ID']
  lastModified: Scalars['Time']
  status: RolloutStatus
}

export type RolloutChangeset = {
  __typename?: 'RolloutChangeset'
  new: Scalars['RawMessage']
  old?: Maybe<Scalars['RawMessage']>
}

export type RolloutEvent = {
  __typename?: 'RolloutEvent'
  created: Scalars['Time']
  data: Scalars['RawMessage']
  id: Scalars['ID']
  type: RolloutEventType
}

export enum RolloutEventType {
  Failed = 'FAILED',
  HelmCompleted = 'HELM_COMPLETED',
  InProgress = 'IN_PROGRESS',
  Processed = 'PROCESSED',
  RolledBack = 'ROLLED_BACK',
  Success = 'SUCCESS',
}

export enum RolloutStatus {
  Deployed = 'DEPLOYED',
  Failed = 'FAILED',
  Pending = 'PENDING',
  Unknown = 'UNKNOWN',
}

export type RolloutSummary = {
  __typename?: 'RolloutSummary'
  created: Scalars['Time']
  feature: Feature
  id: Scalars['ID']
  lastModified: Scalars['Time']
  rollouts: Array<Rollout>
  status: RolloutStatus
}

export type Status = {
  __typename?: 'Status'
  configHash: Scalars['String']
  created: Scalars['Time']
  environmentID: Scalars['ID']
  feature: Scalars['String']
  lastModified: Scalars['Time']
  log: Scalars['String']
  missingDependencies: Array<Feature>
  status: RolloutStatus
  version: Scalars['String']
}

export type Subscription = {
  __typename?: 'Subscription'
  status: Status
}

export type SubscriptionStatusArgs = {
  envID: Scalars['ID']
  feature: Scalars['String']
}

export type Tenant = {
  __typename?: 'Tenant'
  created: Scalars['Time']
  description?: Maybe<Scalars['String']>
  environments: Array<Environment>
  id: Scalars['ID']
  lastModified: Scalars['Time']
  name: Scalars['String']
  warnings: Array<Warning>
}

export type TenantCreate = {
  description?: InputMaybe<Scalars['String']>
  name: Scalars['String']
}

export type UpdateConfiguration = {
  description?: InputMaybe<Scalars['String']>
  value: Scalars['RawMessage']
}

export type Warning = {
  message: Scalars['String']
}

export type UserInfo = {
  __typename?: 'userInfo'
  email: Scalars['String']
}

export type EnvironmentAuditLogQueryVariables = Exact<{
  envID: Scalars['ID']
  featureName?: InputMaybe<Scalars['String']>
}>

export type EnvironmentAuditLogQuery = {
  __typename?: 'Query'
  environment: {
    __typename?: 'Environment'
    id: string
    auditLog: Array<{
      __typename?: 'AuditLog'
      actor: string
      description: string
      objectId: string
      objectType: string
      createdAt: any
    }>
  }
}

export type ConfigurationQueryVariables = Exact<{
  feature: Scalars['String']
  envID?: InputMaybe<Scalars['ID']>
}>

export type ConfigurationQuery = {
  __typename?: 'Query'
  configuration: {
    __typename?: 'EnvConfig'
    configuration: Array<
      | {
          __typename?: 'EnvConfiguration'
          id: string
          description: string
          type: ConfigType
          key: string
          value: any
          displayName: string
          secret: boolean
          chartValue: any
          required: boolean
        }
      | {
          __typename?: 'GlobalConfiguration'
          id: string
          description: string
          type: ConfigType
          key: string
          value: any
          displayName: string
          secret: boolean
          chartValue: any
          required: boolean
        }
    >
    mapping: Array<{
      __typename?: 'MappingValue'
      key: string
      value: any
      displayName: string
    }>
  }
}

export type ConfigurationCreateMutationVariables = Exact<{
  description?: InputMaybe<Scalars['String']>
  feature: Scalars['String']
  key: Scalars['String']
  value: Scalars['RawMessage']
  environmentID?: InputMaybe<Scalars['ID']>
}>

export type ConfigurationCreateMutation = {
  __typename?: 'Mutation'
  configurationCreate:
    | { __typename?: 'EnvConfiguration'; id: string; key: string }
    | { __typename?: 'GlobalConfiguration'; id: string; key: string }
}

export type ConfigurationDeleteMutationVariables = Exact<{
  id: Scalars['ID']
}>

export type ConfigurationDeleteMutation = {
  __typename?: 'Mutation'
  configurationDelete: boolean
}

export type ConfigurationUpdateMutationVariables = Exact<{
  description?: InputMaybe<Scalars['String']>
  id: Scalars['ID']
  value: Scalars['RawMessage']
}>

export type ConfigurationUpdateMutation = {
  __typename?: 'Mutation'
  configurationUpdate:
    | { __typename?: 'EnvConfiguration'; id: string; key: string }
    | { __typename?: 'GlobalConfiguration'; id: string; key: string }
}

export type EnvironmentGetByNamesQueryVariables = Exact<{
  tenantName: Scalars['String']
  environmentName: Scalars['String']
}>

export type EnvironmentGetByNamesQuery = {
  __typename?: 'Query'
  environmentByNames: {
    __typename?: 'Environment'
    id: string
    kind: EnvironmentKind
    name: string
    description?: string | null
    lastModified: any
    created: any
    featureStates: Array<{
      __typename?: 'FeatureState'
      enabled: boolean
      rolloutStatus: RolloutStatus
      feature: { __typename?: 'Feature'; name: string }
    }>
    health: { __typename?: 'Health'; reportedAt: any }
    warnings: Array<
      | {
          __typename?: 'FeatureWarning'
          message: string
          feature: { __typename?: 'Feature'; name: string }
        }
      | { __typename?: 'NaisdWarning'; message: string }
    >
    values: Array<{ __typename?: 'EnvironmentValue'; key: string; value: any }>
  }
}

export type EnvironmentGetQueryVariables = Exact<{
  id: Scalars['ID']
}>

export type EnvironmentGetQuery = {
  __typename?: 'Query'
  environment: {
    __typename?: 'Environment'
    id: string
    name: string
    description?: string | null
    lastModified: any
    created: any
    kind: EnvironmentKind
    values: Array<{ __typename?: 'EnvironmentValue'; key: string; value: any }>
    featureStates: Array<{
      __typename?: 'FeatureState'
      enabled: boolean
      lastModified?: any | null
      created?: any | null
      rolloutStatus: RolloutStatus
      feature: {
        __typename?: 'Feature'
        name: string
        version: string
        chart: string
        repo: string
        source: string
        config: any
        dependsOn: Array<{
          __typename?: 'Dependency'
          anyOf: Array<string>
          allOf: Array<string>
        }>
      }
    }>
  }
}

export type EnvironmentGetReportQueryVariables = Exact<{
  id: Scalars['ID']
}>

export type EnvironmentGetReportQuery = {
  __typename?: 'Query'
  environment: {
    __typename?: 'Environment'
    id: string
    health: { __typename?: 'Health'; reportedAt: any }
    releases: Array<{
      __typename?: 'Release'
      name: string
      status: string
      lastDeployed: any
      version: string
      feature?: { __typename?: 'Feature'; name: string } | null
    }>
    nodes: Array<{
      __typename?: 'KubernetesNode'
      name: string
      kubeletVersion: string
      internalIP: string
      conditions: Array<{
        __typename?: 'KubernetesNodeCondition'
        type: KubernetesNodeConditionType
        status: ConditionStatus
      }>
    }>
  }
}

export type EnvironmentKubernetesNodesQueryVariables = Exact<{
  id: Scalars['ID']
}>

export type EnvironmentKubernetesNodesQuery = {
  __typename?: 'Query'
  environment: {
    __typename?: 'Environment'
    id: string
    nodes: Array<{
      __typename?: 'KubernetesNode'
      name: string
      kubeletVersion: string
      internalIP: string
      conditions: Array<{
        __typename?: 'KubernetesNodeCondition'
        type: KubernetesNodeConditionType
        status: ConditionStatus
      }>
    }>
  }
}

export type EnvironmentHelmInstallsQueryVariables = Exact<{
  id: Scalars['ID']
}>

export type EnvironmentHelmInstallsQuery = {
  __typename?: 'Query'
  environment: {
    __typename?: 'Environment'
    id: string
    releases: Array<{
      __typename?: 'Release'
      name: string
      status: string
      lastDeployed: any
      version: string
      feature?: { __typename?: 'Feature'; name: string } | null
    }>
  }
}

export type EnvironmentUpdateMutationVariables = Exact<{
  description?: InputMaybe<Scalars['String']>
  id: Scalars['ID']
}>

export type EnvironmentUpdateMutation = {
  __typename?: 'Mutation'
  environmentUpdate: { __typename?: 'Environment'; id: string }
}

export type EnvironmentsGetQueryVariables = Exact<{
  tenantID: Scalars['ID']
}>

export type EnvironmentsGetQuery = {
  __typename?: 'Query'
  environments: Array<{ __typename?: 'Environment'; id: string; name: string }>
}

export type FeatureListQueryVariables = Exact<{ [key: string]: never }>

export type FeatureListQuery = {
  __typename?: 'Query'
  features: Array<{
    __typename?: 'Feature'
    name: string
    outdatedInfo: Array<{
      __typename?: 'OutdatedInfo'
      newVersion: string
      dependency: boolean
      dependencyName: string
    } | null>
  }>
}

export type FeatureDetailsQueryVariables = Exact<{
  name: Scalars['String']
}>

export type FeatureDetailsQuery = {
  __typename?: 'Query'
  feature: {
    __typename?: 'Feature'
    name: string
    chart: string
    config: any
    repo: string
    source: string
    environmentKinds: Array<EnvironmentKind>
    version: string
    dependsOn: Array<{
      __typename?: 'Dependency'
      anyOf: Array<string>
      allOf: Array<string>
    }>
    rolloutSummaries: Array<{
      __typename?: 'RolloutSummary'
      id: string
      status: RolloutStatus
      created: any
    }>
    configoverrides: Array<{
      __typename?: 'ConfigOverride'
      keys: Array<string>
      environment: {
        __typename?: 'Environment'
        name: string
        tenant: { __typename?: 'Tenant'; name: string }
      }
    }>
    outdatedInfo: Array<{
      __typename?: 'OutdatedInfo'
      newVersion: string
      dependency: boolean
      dependencyName: string
    } | null>
  }
}

export type FeaturesQueryVariables = Exact<{
  kind?: InputMaybe<EnvironmentKind>
}>

export type FeaturesQuery = {
  __typename?: 'Query'
  features: Array<{
    __typename?: 'Feature'
    name: string
    chart: string
    config: any
    repo: string
    source: string
    environmentKinds: Array<EnvironmentKind>
    version: string
    dependsOn: Array<{
      __typename?: 'Dependency'
      anyOf: Array<string>
      allOf: Array<string>
    }>
  }>
}

export type FeatureStateQueryVariables = Exact<{
  feature: Scalars['String']
  envID: Scalars['ID']
}>

export type FeatureStateQuery = {
  __typename?: 'Query'
  featureState: {
    __typename?: 'FeatureState'
    enabled: boolean
    rolloutStatus: RolloutStatus
    missingDependencies: Array<{ __typename?: 'Feature'; name: string }>
    feature: {
      __typename?: 'Feature'
      name: string
      chart: string
      repo: string
      source: string
      version: string
      dependsOn: Array<{
        __typename?: 'Dependency'
        anyOf: Array<string>
        allOf: Array<string>
      }>
    }
    configuration: {
      __typename?: 'EnvConfig'
      configuration: Array<
        | {
            __typename?: 'EnvConfiguration'
            id: string
            description: string
            type: ConfigType
            key: string
            value: any
            displayName: string
            secret: boolean
            chartValue: any
            required: boolean
          }
        | {
            __typename?: 'GlobalConfiguration'
            id: string
            description: string
            type: ConfigType
            key: string
            value: any
            displayName: string
            secret: boolean
            chartValue: any
            required: boolean
          }
      >
      mapping: Array<{
        __typename?: 'MappingValue'
        key: string
        value: any
        displayName: string
      }>
    }
  }
}

export type FeatureStateSaveMutationVariables = Exact<{
  envID: Scalars['ID']
  feature: Scalars['String']
  enabled: Scalars['Boolean']
}>

export type FeatureStateSaveMutation = {
  __typename?: 'Mutation'
  featureStateSave: { __typename?: 'FeatureState'; enabled: boolean }
}

export type HelmValuesQueryVariables = Exact<{
  feature: Scalars['String']
  envID: Scalars['ID']
}>

export type HelmValuesQuery = { __typename?: 'Query'; helmValues: any }

export type TenantsListQueryVariables = Exact<{ [key: string]: never }>

export type TenantsListQuery = {
  __typename?: 'Query'
  tenants: Array<{
    __typename?: 'Tenant'
    name: string
    warnings: Array<
      | { __typename?: 'FeatureWarning'; message: string }
      | { __typename?: 'NaisdWarning'; message: string }
    >
  }>
  outdatedInfo: Array<{
    __typename?: 'OutdatedInfo'
    dependency: boolean
  } | null>
}

export type RolloutSummaryQueryVariables = Exact<{
  id: Scalars['ID']
}>

export type RolloutSummaryQuery = {
  __typename?: 'Query'
  rolloutSummary: {
    __typename?: 'RolloutSummary'
    id: string
    status: RolloutStatus
    created: any
    feature: { __typename?: 'Feature'; name: string }
    rollouts: Array<{
      __typename?: 'Rollout'
      id: string
      created: any
      status: RolloutStatus
      environment: { __typename?: 'Environment'; kind: EnvironmentKind }
      events: Array<{
        __typename?: 'RolloutEvent'
        id: string
        type: RolloutEventType
        data: any
        created: any
      }>
      changeset: { __typename?: 'RolloutChangeset'; new: any }
    }>
  }
}

export type FeatureStatusQueryVariables = Exact<{
  envID: Scalars['ID']
  feature: Scalars['String']
}>

export type FeatureStatusQuery = {
  __typename?: 'Query'
  featureStatus: {
    __typename?: 'Status'
    environmentID: string
    feature: string
    version: string
    status: RolloutStatus
    configHash: string
    created: any
    lastModified: any
    missingDependencies: Array<{ __typename?: 'Feature'; name: string }>
  }
}

export type FeatureLogsQueryVariables = Exact<{
  envID: Scalars['ID']
  feature: Scalars['String']
}>

export type FeatureLogsQuery = {
  __typename?: 'Query'
  featureStatus: { __typename?: 'Status'; log: string }
}

export type TenantGetQueryVariables = Exact<{
  id: Scalars['ID']
}>

export type TenantGetQuery = {
  __typename?: 'Query'
  tenant: {
    __typename?: 'Tenant'
    id: string
    name: string
    description?: string | null
    created: any
    lastModified: any
  }
}

export type TenantGetByNameQueryVariables = Exact<{
  slug: Scalars['String']
}>

export type TenantGetByNameQuery = {
  __typename?: 'Query'
  tenant: {
    __typename?: 'Tenant'
    id: string
    name: string
    description?: string | null
    created: any
    lastModified: any
    environments: Array<{
      __typename?: 'Environment'
      id: string
      name: string
      warnings: Array<
        | { __typename?: 'FeatureWarning'; message: string }
        | { __typename?: 'NaisdWarning'; message: string }
      >
    }>
    warnings: Array<
      | {
          __typename?: 'FeatureWarning'
          message: string
          feature: { __typename?: 'Feature'; name: string }
          environment: { __typename?: 'Environment'; name: string }
        }
      | {
          __typename?: 'NaisdWarning'
          message: string
          environment: { __typename?: 'Environment'; name: string }
        }
    >
  }
}

export type TenantsGetQueryVariables = Exact<{ [key: string]: never }>

export type TenantsGetQuery = {
  __typename?: 'Query'
  tenants: Array<{
    __typename?: 'Tenant'
    id: string
    name: string
    description?: string | null
    created: any
    lastModified: any
    warnings: Array<
      | { __typename?: 'FeatureWarning'; message: string }
      | { __typename?: 'NaisdWarning'; message: string }
    >
  }>
  outdatedInfo: Array<{
    __typename?: 'OutdatedInfo'
    dependency: boolean
  } | null>
}

export type UserInfoQueryVariables = Exact<{ [key: string]: never }>

export type UserInfoQuery = {
  __typename?: 'Query'
  userInfo?: { __typename?: 'userInfo'; email: string } | null
}

export const EnvironmentAuditLogDocument = gql`
  query EnvironmentAuditLog($envID: ID!, $featureName: String) {
    environment(id: $envID) {
      id
      auditLog(featureName: $featureName) {
        actor
        description
        objectId
        objectType
        createdAt
      }
    }
  }
`

/**
 * __useEnvironmentAuditLogQuery__
 *
 * To run a query within a React component, call `useEnvironmentAuditLogQuery` and pass it any options that fit your needs.
 * When your component renders, `useEnvironmentAuditLogQuery` returns an object from Apollo Client that contains loading, error, and data properties
 * you can use to render your UI.
 *
 * @param baseOptions options that will be passed into the query, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options;
 *
 * @example
 * const { data, loading, error } = useEnvironmentAuditLogQuery({
 *   variables: {
 *      envID: // value for 'envID'
 *      featureName: // value for 'featureName'
 *   },
 * });
 */
export function useEnvironmentAuditLogQuery(
  baseOptions: Apollo.QueryHookOptions<
    EnvironmentAuditLogQuery,
    EnvironmentAuditLogQueryVariables
  >,
) {
  const options = { ...defaultOptions, ...baseOptions }
  return Apollo.useQuery<
    EnvironmentAuditLogQuery,
    EnvironmentAuditLogQueryVariables
  >(EnvironmentAuditLogDocument, options)
}
export function useEnvironmentAuditLogLazyQuery(
  baseOptions?: Apollo.LazyQueryHookOptions<
    EnvironmentAuditLogQuery,
    EnvironmentAuditLogQueryVariables
  >,
) {
  const options = { ...defaultOptions, ...baseOptions }
  return Apollo.useLazyQuery<
    EnvironmentAuditLogQuery,
    EnvironmentAuditLogQueryVariables
  >(EnvironmentAuditLogDocument, options)
}
export type EnvironmentAuditLogQueryHookResult = ReturnType<
  typeof useEnvironmentAuditLogQuery
>
export type EnvironmentAuditLogLazyQueryHookResult = ReturnType<
  typeof useEnvironmentAuditLogLazyQuery
>
export type EnvironmentAuditLogQueryResult = Apollo.QueryResult<
  EnvironmentAuditLogQuery,
  EnvironmentAuditLogQueryVariables
>
export const ConfigurationDocument = gql`
  query configuration($feature: String!, $envID: ID) {
    configuration(feature: $feature, envID: $envID) {
      configuration {
        id
        description
        type
        key
        value
        displayName
        secret
        chartValue
        required
      }
      mapping {
        key
        value
        displayName
      }
    }
  }
`

/**
 * __useConfigurationQuery__
 *
 * To run a query within a React component, call `useConfigurationQuery` and pass it any options that fit your needs.
 * When your component renders, `useConfigurationQuery` returns an object from Apollo Client that contains loading, error, and data properties
 * you can use to render your UI.
 *
 * @param baseOptions options that will be passed into the query, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options;
 *
 * @example
 * const { data, loading, error } = useConfigurationQuery({
 *   variables: {
 *      feature: // value for 'feature'
 *      envID: // value for 'envID'
 *   },
 * });
 */
export function useConfigurationQuery(
  baseOptions: Apollo.QueryHookOptions<
    ConfigurationQuery,
    ConfigurationQueryVariables
  >,
) {
  const options = { ...defaultOptions, ...baseOptions }
  return Apollo.useQuery<ConfigurationQuery, ConfigurationQueryVariables>(
    ConfigurationDocument,
    options,
  )
}
export function useConfigurationLazyQuery(
  baseOptions?: Apollo.LazyQueryHookOptions<
    ConfigurationQuery,
    ConfigurationQueryVariables
  >,
) {
  const options = { ...defaultOptions, ...baseOptions }
  return Apollo.useLazyQuery<ConfigurationQuery, ConfigurationQueryVariables>(
    ConfigurationDocument,
    options,
  )
}
export type ConfigurationQueryHookResult = ReturnType<
  typeof useConfigurationQuery
>
export type ConfigurationLazyQueryHookResult = ReturnType<
  typeof useConfigurationLazyQuery
>
export type ConfigurationQueryResult = Apollo.QueryResult<
  ConfigurationQuery,
  ConfigurationQueryVariables
>
export const ConfigurationCreateDocument = gql`
  mutation configurationCreate(
    $description: String
    $feature: String!
    $key: String!
    $value: RawMessage!
    $environmentID: ID
  ) {
    configurationCreate(
      configuration: {
        feature: $feature
        description: $description
        key: $key
        value: $value
        environmentID: $environmentID
      }
    ) {
      id
      key
    }
  }
`
export type ConfigurationCreateMutationFn = Apollo.MutationFunction<
  ConfigurationCreateMutation,
  ConfigurationCreateMutationVariables
>

/**
 * __useConfigurationCreateMutation__
 *
 * To run a mutation, you first call `useConfigurationCreateMutation` within a React component and pass it any options that fit your needs.
 * When your component renders, `useConfigurationCreateMutation` returns a tuple that includes:
 * - A mutate function that you can call at any time to execute the mutation
 * - An object with fields that represent the current status of the mutation's execution
 *
 * @param baseOptions options that will be passed into the mutation, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options-2;
 *
 * @example
 * const [configurationCreateMutation, { data, loading, error }] = useConfigurationCreateMutation({
 *   variables: {
 *      description: // value for 'description'
 *      feature: // value for 'feature'
 *      key: // value for 'key'
 *      value: // value for 'value'
 *      environmentID: // value for 'environmentID'
 *   },
 * });
 */
export function useConfigurationCreateMutation(
  baseOptions?: Apollo.MutationHookOptions<
    ConfigurationCreateMutation,
    ConfigurationCreateMutationVariables
  >,
) {
  const options = { ...defaultOptions, ...baseOptions }
  return Apollo.useMutation<
    ConfigurationCreateMutation,
    ConfigurationCreateMutationVariables
  >(ConfigurationCreateDocument, options)
}
export type ConfigurationCreateMutationHookResult = ReturnType<
  typeof useConfigurationCreateMutation
>
export type ConfigurationCreateMutationResult =
  Apollo.MutationResult<ConfigurationCreateMutation>
export type ConfigurationCreateMutationOptions = Apollo.BaseMutationOptions<
  ConfigurationCreateMutation,
  ConfigurationCreateMutationVariables
>
export const ConfigurationDeleteDocument = gql`
  mutation configurationDelete($id: ID!) {
    configurationDelete(id: $id)
  }
`
export type ConfigurationDeleteMutationFn = Apollo.MutationFunction<
  ConfigurationDeleteMutation,
  ConfigurationDeleteMutationVariables
>

/**
 * __useConfigurationDeleteMutation__
 *
 * To run a mutation, you first call `useConfigurationDeleteMutation` within a React component and pass it any options that fit your needs.
 * When your component renders, `useConfigurationDeleteMutation` returns a tuple that includes:
 * - A mutate function that you can call at any time to execute the mutation
 * - An object with fields that represent the current status of the mutation's execution
 *
 * @param baseOptions options that will be passed into the mutation, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options-2;
 *
 * @example
 * const [configurationDeleteMutation, { data, loading, error }] = useConfigurationDeleteMutation({
 *   variables: {
 *      id: // value for 'id'
 *   },
 * });
 */
export function useConfigurationDeleteMutation(
  baseOptions?: Apollo.MutationHookOptions<
    ConfigurationDeleteMutation,
    ConfigurationDeleteMutationVariables
  >,
) {
  const options = { ...defaultOptions, ...baseOptions }
  return Apollo.useMutation<
    ConfigurationDeleteMutation,
    ConfigurationDeleteMutationVariables
  >(ConfigurationDeleteDocument, options)
}
export type ConfigurationDeleteMutationHookResult = ReturnType<
  typeof useConfigurationDeleteMutation
>
export type ConfigurationDeleteMutationResult =
  Apollo.MutationResult<ConfigurationDeleteMutation>
export type ConfigurationDeleteMutationOptions = Apollo.BaseMutationOptions<
  ConfigurationDeleteMutation,
  ConfigurationDeleteMutationVariables
>
export const ConfigurationUpdateDocument = gql`
  mutation configurationUpdate(
    $description: String
    $id: ID!
    $value: RawMessage!
  ) {
    configurationUpdate(
      id: $id
      configuration: { description: $description, value: $value }
    ) {
      id
      key
    }
  }
`
export type ConfigurationUpdateMutationFn = Apollo.MutationFunction<
  ConfigurationUpdateMutation,
  ConfigurationUpdateMutationVariables
>

/**
 * __useConfigurationUpdateMutation__
 *
 * To run a mutation, you first call `useConfigurationUpdateMutation` within a React component and pass it any options that fit your needs.
 * When your component renders, `useConfigurationUpdateMutation` returns a tuple that includes:
 * - A mutate function that you can call at any time to execute the mutation
 * - An object with fields that represent the current status of the mutation's execution
 *
 * @param baseOptions options that will be passed into the mutation, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options-2;
 *
 * @example
 * const [configurationUpdateMutation, { data, loading, error }] = useConfigurationUpdateMutation({
 *   variables: {
 *      description: // value for 'description'
 *      id: // value for 'id'
 *      value: // value for 'value'
 *   },
 * });
 */
export function useConfigurationUpdateMutation(
  baseOptions?: Apollo.MutationHookOptions<
    ConfigurationUpdateMutation,
    ConfigurationUpdateMutationVariables
  >,
) {
  const options = { ...defaultOptions, ...baseOptions }
  return Apollo.useMutation<
    ConfigurationUpdateMutation,
    ConfigurationUpdateMutationVariables
  >(ConfigurationUpdateDocument, options)
}
export type ConfigurationUpdateMutationHookResult = ReturnType<
  typeof useConfigurationUpdateMutation
>
export type ConfigurationUpdateMutationResult =
  Apollo.MutationResult<ConfigurationUpdateMutation>
export type ConfigurationUpdateMutationOptions = Apollo.BaseMutationOptions<
  ConfigurationUpdateMutation,
  ConfigurationUpdateMutationVariables
>
export const EnvironmentGetByNamesDocument = gql`
  query environmentGetByNames($tenantName: String!, $environmentName: String!) {
    environmentByNames(
      tenantName: $tenantName
      environmentName: $environmentName
    ) {
      id
      kind
      name
      description
      lastModified
      created
      featureStates {
        enabled
        rolloutStatus
        feature {
          name
        }
      }
      health {
        reportedAt
      }
      warnings {
        message
        ... on FeatureWarning {
          feature {
            name
          }
        }
      }
      values {
        key
        value
      }
    }
  }
`

/**
 * __useEnvironmentGetByNamesQuery__
 *
 * To run a query within a React component, call `useEnvironmentGetByNamesQuery` and pass it any options that fit your needs.
 * When your component renders, `useEnvironmentGetByNamesQuery` returns an object from Apollo Client that contains loading, error, and data properties
 * you can use to render your UI.
 *
 * @param baseOptions options that will be passed into the query, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options;
 *
 * @example
 * const { data, loading, error } = useEnvironmentGetByNamesQuery({
 *   variables: {
 *      tenantName: // value for 'tenantName'
 *      environmentName: // value for 'environmentName'
 *   },
 * });
 */
export function useEnvironmentGetByNamesQuery(
  baseOptions: Apollo.QueryHookOptions<
    EnvironmentGetByNamesQuery,
    EnvironmentGetByNamesQueryVariables
  >,
) {
  const options = { ...defaultOptions, ...baseOptions }
  return Apollo.useQuery<
    EnvironmentGetByNamesQuery,
    EnvironmentGetByNamesQueryVariables
  >(EnvironmentGetByNamesDocument, options)
}
export function useEnvironmentGetByNamesLazyQuery(
  baseOptions?: Apollo.LazyQueryHookOptions<
    EnvironmentGetByNamesQuery,
    EnvironmentGetByNamesQueryVariables
  >,
) {
  const options = { ...defaultOptions, ...baseOptions }
  return Apollo.useLazyQuery<
    EnvironmentGetByNamesQuery,
    EnvironmentGetByNamesQueryVariables
  >(EnvironmentGetByNamesDocument, options)
}
export type EnvironmentGetByNamesQueryHookResult = ReturnType<
  typeof useEnvironmentGetByNamesQuery
>
export type EnvironmentGetByNamesLazyQueryHookResult = ReturnType<
  typeof useEnvironmentGetByNamesLazyQuery
>
export type EnvironmentGetByNamesQueryResult = Apollo.QueryResult<
  EnvironmentGetByNamesQuery,
  EnvironmentGetByNamesQueryVariables
>
export const EnvironmentGetDocument = gql`
  query environmentGet($id: ID!) {
    environment(id: $id) {
      id
      name
      description
      lastModified
      created
      kind
      values {
        key
        value
      }
      featureStates {
        enabled
        lastModified
        created
        rolloutStatus
        feature {
          name
          version
          chart
          repo
          source
          config
          dependsOn {
            anyOf
            allOf
          }
        }
      }
    }
  }
`

/**
 * __useEnvironmentGetQuery__
 *
 * To run a query within a React component, call `useEnvironmentGetQuery` and pass it any options that fit your needs.
 * When your component renders, `useEnvironmentGetQuery` returns an object from Apollo Client that contains loading, error, and data properties
 * you can use to render your UI.
 *
 * @param baseOptions options that will be passed into the query, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options;
 *
 * @example
 * const { data, loading, error } = useEnvironmentGetQuery({
 *   variables: {
 *      id: // value for 'id'
 *   },
 * });
 */
export function useEnvironmentGetQuery(
  baseOptions: Apollo.QueryHookOptions<
    EnvironmentGetQuery,
    EnvironmentGetQueryVariables
  >,
) {
  const options = { ...defaultOptions, ...baseOptions }
  return Apollo.useQuery<EnvironmentGetQuery, EnvironmentGetQueryVariables>(
    EnvironmentGetDocument,
    options,
  )
}
export function useEnvironmentGetLazyQuery(
  baseOptions?: Apollo.LazyQueryHookOptions<
    EnvironmentGetQuery,
    EnvironmentGetQueryVariables
  >,
) {
  const options = { ...defaultOptions, ...baseOptions }
  return Apollo.useLazyQuery<EnvironmentGetQuery, EnvironmentGetQueryVariables>(
    EnvironmentGetDocument,
    options,
  )
}
export type EnvironmentGetQueryHookResult = ReturnType<
  typeof useEnvironmentGetQuery
>
export type EnvironmentGetLazyQueryHookResult = ReturnType<
  typeof useEnvironmentGetLazyQuery
>
export type EnvironmentGetQueryResult = Apollo.QueryResult<
  EnvironmentGetQuery,
  EnvironmentGetQueryVariables
>
export const EnvironmentGetReportDocument = gql`
  query environmentGetReport($id: ID!) {
    environment(id: $id) {
      id
      health {
        reportedAt
      }
      releases {
        name
        feature {
          name
        }
        status
        lastDeployed
        version
      }
      nodes {
        name
        kubeletVersion
        internalIP
        conditions {
          type
          status
        }
      }
    }
  }
`

/**
 * __useEnvironmentGetReportQuery__
 *
 * To run a query within a React component, call `useEnvironmentGetReportQuery` and pass it any options that fit your needs.
 * When your component renders, `useEnvironmentGetReportQuery` returns an object from Apollo Client that contains loading, error, and data properties
 * you can use to render your UI.
 *
 * @param baseOptions options that will be passed into the query, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options;
 *
 * @example
 * const { data, loading, error } = useEnvironmentGetReportQuery({
 *   variables: {
 *      id: // value for 'id'
 *   },
 * });
 */
export function useEnvironmentGetReportQuery(
  baseOptions: Apollo.QueryHookOptions<
    EnvironmentGetReportQuery,
    EnvironmentGetReportQueryVariables
  >,
) {
  const options = { ...defaultOptions, ...baseOptions }
  return Apollo.useQuery<
    EnvironmentGetReportQuery,
    EnvironmentGetReportQueryVariables
  >(EnvironmentGetReportDocument, options)
}
export function useEnvironmentGetReportLazyQuery(
  baseOptions?: Apollo.LazyQueryHookOptions<
    EnvironmentGetReportQuery,
    EnvironmentGetReportQueryVariables
  >,
) {
  const options = { ...defaultOptions, ...baseOptions }
  return Apollo.useLazyQuery<
    EnvironmentGetReportQuery,
    EnvironmentGetReportQueryVariables
  >(EnvironmentGetReportDocument, options)
}
export type EnvironmentGetReportQueryHookResult = ReturnType<
  typeof useEnvironmentGetReportQuery
>
export type EnvironmentGetReportLazyQueryHookResult = ReturnType<
  typeof useEnvironmentGetReportLazyQuery
>
export type EnvironmentGetReportQueryResult = Apollo.QueryResult<
  EnvironmentGetReportQuery,
  EnvironmentGetReportQueryVariables
>
export const EnvironmentKubernetesNodesDocument = gql`
  query environmentKubernetesNodes($id: ID!) {
    environment(id: $id) {
      id
      nodes {
        name
        kubeletVersion
        internalIP
        conditions {
          type
          status
        }
      }
    }
  }
`

/**
 * __useEnvironmentKubernetesNodesQuery__
 *
 * To run a query within a React component, call `useEnvironmentKubernetesNodesQuery` and pass it any options that fit your needs.
 * When your component renders, `useEnvironmentKubernetesNodesQuery` returns an object from Apollo Client that contains loading, error, and data properties
 * you can use to render your UI.
 *
 * @param baseOptions options that will be passed into the query, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options;
 *
 * @example
 * const { data, loading, error } = useEnvironmentKubernetesNodesQuery({
 *   variables: {
 *      id: // value for 'id'
 *   },
 * });
 */
export function useEnvironmentKubernetesNodesQuery(
  baseOptions: Apollo.QueryHookOptions<
    EnvironmentKubernetesNodesQuery,
    EnvironmentKubernetesNodesQueryVariables
  >,
) {
  const options = { ...defaultOptions, ...baseOptions }
  return Apollo.useQuery<
    EnvironmentKubernetesNodesQuery,
    EnvironmentKubernetesNodesQueryVariables
  >(EnvironmentKubernetesNodesDocument, options)
}
export function useEnvironmentKubernetesNodesLazyQuery(
  baseOptions?: Apollo.LazyQueryHookOptions<
    EnvironmentKubernetesNodesQuery,
    EnvironmentKubernetesNodesQueryVariables
  >,
) {
  const options = { ...defaultOptions, ...baseOptions }
  return Apollo.useLazyQuery<
    EnvironmentKubernetesNodesQuery,
    EnvironmentKubernetesNodesQueryVariables
  >(EnvironmentKubernetesNodesDocument, options)
}
export type EnvironmentKubernetesNodesQueryHookResult = ReturnType<
  typeof useEnvironmentKubernetesNodesQuery
>
export type EnvironmentKubernetesNodesLazyQueryHookResult = ReturnType<
  typeof useEnvironmentKubernetesNodesLazyQuery
>
export type EnvironmentKubernetesNodesQueryResult = Apollo.QueryResult<
  EnvironmentKubernetesNodesQuery,
  EnvironmentKubernetesNodesQueryVariables
>
export const EnvironmentHelmInstallsDocument = gql`
  query environmentHelmInstalls($id: ID!) {
    environment(id: $id) {
      id
      releases {
        name
        feature {
          name
        }
        status
        lastDeployed
        version
      }
    }
  }
`

/**
 * __useEnvironmentHelmInstallsQuery__
 *
 * To run a query within a React component, call `useEnvironmentHelmInstallsQuery` and pass it any options that fit your needs.
 * When your component renders, `useEnvironmentHelmInstallsQuery` returns an object from Apollo Client that contains loading, error, and data properties
 * you can use to render your UI.
 *
 * @param baseOptions options that will be passed into the query, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options;
 *
 * @example
 * const { data, loading, error } = useEnvironmentHelmInstallsQuery({
 *   variables: {
 *      id: // value for 'id'
 *   },
 * });
 */
export function useEnvironmentHelmInstallsQuery(
  baseOptions: Apollo.QueryHookOptions<
    EnvironmentHelmInstallsQuery,
    EnvironmentHelmInstallsQueryVariables
  >,
) {
  const options = { ...defaultOptions, ...baseOptions }
  return Apollo.useQuery<
    EnvironmentHelmInstallsQuery,
    EnvironmentHelmInstallsQueryVariables
  >(EnvironmentHelmInstallsDocument, options)
}
export function useEnvironmentHelmInstallsLazyQuery(
  baseOptions?: Apollo.LazyQueryHookOptions<
    EnvironmentHelmInstallsQuery,
    EnvironmentHelmInstallsQueryVariables
  >,
) {
  const options = { ...defaultOptions, ...baseOptions }
  return Apollo.useLazyQuery<
    EnvironmentHelmInstallsQuery,
    EnvironmentHelmInstallsQueryVariables
  >(EnvironmentHelmInstallsDocument, options)
}
export type EnvironmentHelmInstallsQueryHookResult = ReturnType<
  typeof useEnvironmentHelmInstallsQuery
>
export type EnvironmentHelmInstallsLazyQueryHookResult = ReturnType<
  typeof useEnvironmentHelmInstallsLazyQuery
>
export type EnvironmentHelmInstallsQueryResult = Apollo.QueryResult<
  EnvironmentHelmInstallsQuery,
  EnvironmentHelmInstallsQueryVariables
>
export const EnvironmentUpdateDocument = gql`
  mutation environmentUpdate($description: String, $id: ID!) {
    environmentUpdate(id: $id, input: { description: $description }) {
      id
    }
  }
`
export type EnvironmentUpdateMutationFn = Apollo.MutationFunction<
  EnvironmentUpdateMutation,
  EnvironmentUpdateMutationVariables
>

/**
 * __useEnvironmentUpdateMutation__
 *
 * To run a mutation, you first call `useEnvironmentUpdateMutation` within a React component and pass it any options that fit your needs.
 * When your component renders, `useEnvironmentUpdateMutation` returns a tuple that includes:
 * - A mutate function that you can call at any time to execute the mutation
 * - An object with fields that represent the current status of the mutation's execution
 *
 * @param baseOptions options that will be passed into the mutation, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options-2;
 *
 * @example
 * const [environmentUpdateMutation, { data, loading, error }] = useEnvironmentUpdateMutation({
 *   variables: {
 *      description: // value for 'description'
 *      id: // value for 'id'
 *   },
 * });
 */
export function useEnvironmentUpdateMutation(
  baseOptions?: Apollo.MutationHookOptions<
    EnvironmentUpdateMutation,
    EnvironmentUpdateMutationVariables
  >,
) {
  const options = { ...defaultOptions, ...baseOptions }
  return Apollo.useMutation<
    EnvironmentUpdateMutation,
    EnvironmentUpdateMutationVariables
  >(EnvironmentUpdateDocument, options)
}
export type EnvironmentUpdateMutationHookResult = ReturnType<
  typeof useEnvironmentUpdateMutation
>
export type EnvironmentUpdateMutationResult =
  Apollo.MutationResult<EnvironmentUpdateMutation>
export type EnvironmentUpdateMutationOptions = Apollo.BaseMutationOptions<
  EnvironmentUpdateMutation,
  EnvironmentUpdateMutationVariables
>
export const EnvironmentsGetDocument = gql`
  query environmentsGet($tenantID: ID!) {
    environments(tenantID: $tenantID) {
      id
      name
    }
  }
`

/**
 * __useEnvironmentsGetQuery__
 *
 * To run a query within a React component, call `useEnvironmentsGetQuery` and pass it any options that fit your needs.
 * When your component renders, `useEnvironmentsGetQuery` returns an object from Apollo Client that contains loading, error, and data properties
 * you can use to render your UI.
 *
 * @param baseOptions options that will be passed into the query, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options;
 *
 * @example
 * const { data, loading, error } = useEnvironmentsGetQuery({
 *   variables: {
 *      tenantID: // value for 'tenantID'
 *   },
 * });
 */
export function useEnvironmentsGetQuery(
  baseOptions: Apollo.QueryHookOptions<
    EnvironmentsGetQuery,
    EnvironmentsGetQueryVariables
  >,
) {
  const options = { ...defaultOptions, ...baseOptions }
  return Apollo.useQuery<EnvironmentsGetQuery, EnvironmentsGetQueryVariables>(
    EnvironmentsGetDocument,
    options,
  )
}
export function useEnvironmentsGetLazyQuery(
  baseOptions?: Apollo.LazyQueryHookOptions<
    EnvironmentsGetQuery,
    EnvironmentsGetQueryVariables
  >,
) {
  const options = { ...defaultOptions, ...baseOptions }
  return Apollo.useLazyQuery<
    EnvironmentsGetQuery,
    EnvironmentsGetQueryVariables
  >(EnvironmentsGetDocument, options)
}
export type EnvironmentsGetQueryHookResult = ReturnType<
  typeof useEnvironmentsGetQuery
>
export type EnvironmentsGetLazyQueryHookResult = ReturnType<
  typeof useEnvironmentsGetLazyQuery
>
export type EnvironmentsGetQueryResult = Apollo.QueryResult<
  EnvironmentsGetQuery,
  EnvironmentsGetQueryVariables
>
export const FeatureListDocument = gql`
  query FeatureList {
    features {
      name
      outdatedInfo {
        newVersion
        dependency
        dependencyName
      }
    }
  }
`

/**
 * __useFeatureListQuery__
 *
 * To run a query within a React component, call `useFeatureListQuery` and pass it any options that fit your needs.
 * When your component renders, `useFeatureListQuery` returns an object from Apollo Client that contains loading, error, and data properties
 * you can use to render your UI.
 *
 * @param baseOptions options that will be passed into the query, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options;
 *
 * @example
 * const { data, loading, error } = useFeatureListQuery({
 *   variables: {
 *   },
 * });
 */
export function useFeatureListQuery(
  baseOptions?: Apollo.QueryHookOptions<
    FeatureListQuery,
    FeatureListQueryVariables
  >,
) {
  const options = { ...defaultOptions, ...baseOptions }
  return Apollo.useQuery<FeatureListQuery, FeatureListQueryVariables>(
    FeatureListDocument,
    options,
  )
}
export function useFeatureListLazyQuery(
  baseOptions?: Apollo.LazyQueryHookOptions<
    FeatureListQuery,
    FeatureListQueryVariables
  >,
) {
  const options = { ...defaultOptions, ...baseOptions }
  return Apollo.useLazyQuery<FeatureListQuery, FeatureListQueryVariables>(
    FeatureListDocument,
    options,
  )
}
export type FeatureListQueryHookResult = ReturnType<typeof useFeatureListQuery>
export type FeatureListLazyQueryHookResult = ReturnType<
  typeof useFeatureListLazyQuery
>
export type FeatureListQueryResult = Apollo.QueryResult<
  FeatureListQuery,
  FeatureListQueryVariables
>
export const FeatureDetailsDocument = gql`
  query FeatureDetails($name: String!) {
    feature(name: $name) {
      dependsOn {
        anyOf
        allOf
      }
      name
      chart
      config
      repo
      source
      environmentKinds
      version
      rolloutSummaries {
        id
        status
        created
      }
      configoverrides {
        keys
        environment {
          name
          tenant {
            name
          }
        }
      }
      outdatedInfo {
        newVersion
        dependency
        dependencyName
      }
    }
  }
`

/**
 * __useFeatureDetailsQuery__
 *
 * To run a query within a React component, call `useFeatureDetailsQuery` and pass it any options that fit your needs.
 * When your component renders, `useFeatureDetailsQuery` returns an object from Apollo Client that contains loading, error, and data properties
 * you can use to render your UI.
 *
 * @param baseOptions options that will be passed into the query, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options;
 *
 * @example
 * const { data, loading, error } = useFeatureDetailsQuery({
 *   variables: {
 *      name: // value for 'name'
 *   },
 * });
 */
export function useFeatureDetailsQuery(
  baseOptions: Apollo.QueryHookOptions<
    FeatureDetailsQuery,
    FeatureDetailsQueryVariables
  >,
) {
  const options = { ...defaultOptions, ...baseOptions }
  return Apollo.useQuery<FeatureDetailsQuery, FeatureDetailsQueryVariables>(
    FeatureDetailsDocument,
    options,
  )
}
export function useFeatureDetailsLazyQuery(
  baseOptions?: Apollo.LazyQueryHookOptions<
    FeatureDetailsQuery,
    FeatureDetailsQueryVariables
  >,
) {
  const options = { ...defaultOptions, ...baseOptions }
  return Apollo.useLazyQuery<FeatureDetailsQuery, FeatureDetailsQueryVariables>(
    FeatureDetailsDocument,
    options,
  )
}
export type FeatureDetailsQueryHookResult = ReturnType<
  typeof useFeatureDetailsQuery
>
export type FeatureDetailsLazyQueryHookResult = ReturnType<
  typeof useFeatureDetailsLazyQuery
>
export type FeatureDetailsQueryResult = Apollo.QueryResult<
  FeatureDetailsQuery,
  FeatureDetailsQueryVariables
>
export const FeaturesDocument = gql`
  query Features($kind: EnvironmentKind) {
    features(kind: $kind) {
      dependsOn {
        anyOf
        allOf
      }
      name
      chart
      config
      repo
      source
      environmentKinds
      version
    }
  }
`

/**
 * __useFeaturesQuery__
 *
 * To run a query within a React component, call `useFeaturesQuery` and pass it any options that fit your needs.
 * When your component renders, `useFeaturesQuery` returns an object from Apollo Client that contains loading, error, and data properties
 * you can use to render your UI.
 *
 * @param baseOptions options that will be passed into the query, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options;
 *
 * @example
 * const { data, loading, error } = useFeaturesQuery({
 *   variables: {
 *      kind: // value for 'kind'
 *   },
 * });
 */
export function useFeaturesQuery(
  baseOptions?: Apollo.QueryHookOptions<FeaturesQuery, FeaturesQueryVariables>,
) {
  const options = { ...defaultOptions, ...baseOptions }
  return Apollo.useQuery<FeaturesQuery, FeaturesQueryVariables>(
    FeaturesDocument,
    options,
  )
}
export function useFeaturesLazyQuery(
  baseOptions?: Apollo.LazyQueryHookOptions<
    FeaturesQuery,
    FeaturesQueryVariables
  >,
) {
  const options = { ...defaultOptions, ...baseOptions }
  return Apollo.useLazyQuery<FeaturesQuery, FeaturesQueryVariables>(
    FeaturesDocument,
    options,
  )
}
export type FeaturesQueryHookResult = ReturnType<typeof useFeaturesQuery>
export type FeaturesLazyQueryHookResult = ReturnType<
  typeof useFeaturesLazyQuery
>
export type FeaturesQueryResult = Apollo.QueryResult<
  FeaturesQuery,
  FeaturesQueryVariables
>
export const FeatureStateDocument = gql`
  query FeatureState($feature: String!, $envID: ID!) {
    featureState(feature: $feature, envID: $envID) {
      enabled
      rolloutStatus
      missingDependencies {
        name
      }
      feature {
        name
        dependsOn {
          anyOf
          allOf
        }
        chart
        repo
        source
        version
      }
      configuration {
        configuration {
          id
          description
          type
          key
          value
          displayName
          secret
          chartValue
          required
        }
        mapping {
          key
          value
          displayName
        }
      }
    }
  }
`

/**
 * __useFeatureStateQuery__
 *
 * To run a query within a React component, call `useFeatureStateQuery` and pass it any options that fit your needs.
 * When your component renders, `useFeatureStateQuery` returns an object from Apollo Client that contains loading, error, and data properties
 * you can use to render your UI.
 *
 * @param baseOptions options that will be passed into the query, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options;
 *
 * @example
 * const { data, loading, error } = useFeatureStateQuery({
 *   variables: {
 *      feature: // value for 'feature'
 *      envID: // value for 'envID'
 *   },
 * });
 */
export function useFeatureStateQuery(
  baseOptions: Apollo.QueryHookOptions<
    FeatureStateQuery,
    FeatureStateQueryVariables
  >,
) {
  const options = { ...defaultOptions, ...baseOptions }
  return Apollo.useQuery<FeatureStateQuery, FeatureStateQueryVariables>(
    FeatureStateDocument,
    options,
  )
}
export function useFeatureStateLazyQuery(
  baseOptions?: Apollo.LazyQueryHookOptions<
    FeatureStateQuery,
    FeatureStateQueryVariables
  >,
) {
  const options = { ...defaultOptions, ...baseOptions }
  return Apollo.useLazyQuery<FeatureStateQuery, FeatureStateQueryVariables>(
    FeatureStateDocument,
    options,
  )
}
export type FeatureStateQueryHookResult = ReturnType<
  typeof useFeatureStateQuery
>
export type FeatureStateLazyQueryHookResult = ReturnType<
  typeof useFeatureStateLazyQuery
>
export type FeatureStateQueryResult = Apollo.QueryResult<
  FeatureStateQuery,
  FeatureStateQueryVariables
>
export const FeatureStateSaveDocument = gql`
  mutation featureStateSave(
    $envID: ID!
    $feature: String!
    $enabled: Boolean!
  ) {
    featureStateSave(envID: $envID, feature: $feature, enabled: $enabled) {
      enabled
    }
  }
`
export type FeatureStateSaveMutationFn = Apollo.MutationFunction<
  FeatureStateSaveMutation,
  FeatureStateSaveMutationVariables
>

/**
 * __useFeatureStateSaveMutation__
 *
 * To run a mutation, you first call `useFeatureStateSaveMutation` within a React component and pass it any options that fit your needs.
 * When your component renders, `useFeatureStateSaveMutation` returns a tuple that includes:
 * - A mutate function that you can call at any time to execute the mutation
 * - An object with fields that represent the current status of the mutation's execution
 *
 * @param baseOptions options that will be passed into the mutation, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options-2;
 *
 * @example
 * const [featureStateSaveMutation, { data, loading, error }] = useFeatureStateSaveMutation({
 *   variables: {
 *      envID: // value for 'envID'
 *      feature: // value for 'feature'
 *      enabled: // value for 'enabled'
 *   },
 * });
 */
export function useFeatureStateSaveMutation(
  baseOptions?: Apollo.MutationHookOptions<
    FeatureStateSaveMutation,
    FeatureStateSaveMutationVariables
  >,
) {
  const options = { ...defaultOptions, ...baseOptions }
  return Apollo.useMutation<
    FeatureStateSaveMutation,
    FeatureStateSaveMutationVariables
  >(FeatureStateSaveDocument, options)
}
export type FeatureStateSaveMutationHookResult = ReturnType<
  typeof useFeatureStateSaveMutation
>
export type FeatureStateSaveMutationResult =
  Apollo.MutationResult<FeatureStateSaveMutation>
export type FeatureStateSaveMutationOptions = Apollo.BaseMutationOptions<
  FeatureStateSaveMutation,
  FeatureStateSaveMutationVariables
>
export const HelmValuesDocument = gql`
  query helmValues($feature: String!, $envID: ID!) {
    helmValues(feature: $feature, envID: $envID)
  }
`

/**
 * __useHelmValuesQuery__
 *
 * To run a query within a React component, call `useHelmValuesQuery` and pass it any options that fit your needs.
 * When your component renders, `useHelmValuesQuery` returns an object from Apollo Client that contains loading, error, and data properties
 * you can use to render your UI.
 *
 * @param baseOptions options that will be passed into the query, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options;
 *
 * @example
 * const { data, loading, error } = useHelmValuesQuery({
 *   variables: {
 *      feature: // value for 'feature'
 *      envID: // value for 'envID'
 *   },
 * });
 */
export function useHelmValuesQuery(
  baseOptions: Apollo.QueryHookOptions<
    HelmValuesQuery,
    HelmValuesQueryVariables
  >,
) {
  const options = { ...defaultOptions, ...baseOptions }
  return Apollo.useQuery<HelmValuesQuery, HelmValuesQueryVariables>(
    HelmValuesDocument,
    options,
  )
}
export function useHelmValuesLazyQuery(
  baseOptions?: Apollo.LazyQueryHookOptions<
    HelmValuesQuery,
    HelmValuesQueryVariables
  >,
) {
  const options = { ...defaultOptions, ...baseOptions }
  return Apollo.useLazyQuery<HelmValuesQuery, HelmValuesQueryVariables>(
    HelmValuesDocument,
    options,
  )
}
export type HelmValuesQueryHookResult = ReturnType<typeof useHelmValuesQuery>
export type HelmValuesLazyQueryHookResult = ReturnType<
  typeof useHelmValuesLazyQuery
>
export type HelmValuesQueryResult = Apollo.QueryResult<
  HelmValuesQuery,
  HelmValuesQueryVariables
>
export const TenantsListDocument = gql`
  query TenantsList {
    tenants {
      name
      warnings {
        message
      }
    }
    outdatedInfo {
      dependency
    }
  }
`

/**
 * __useTenantsListQuery__
 *
 * To run a query within a React component, call `useTenantsListQuery` and pass it any options that fit your needs.
 * When your component renders, `useTenantsListQuery` returns an object from Apollo Client that contains loading, error, and data properties
 * you can use to render your UI.
 *
 * @param baseOptions options that will be passed into the query, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options;
 *
 * @example
 * const { data, loading, error } = useTenantsListQuery({
 *   variables: {
 *   },
 * });
 */
export function useTenantsListQuery(
  baseOptions?: Apollo.QueryHookOptions<
    TenantsListQuery,
    TenantsListQueryVariables
  >,
) {
  const options = { ...defaultOptions, ...baseOptions }
  return Apollo.useQuery<TenantsListQuery, TenantsListQueryVariables>(
    TenantsListDocument,
    options,
  )
}
export function useTenantsListLazyQuery(
  baseOptions?: Apollo.LazyQueryHookOptions<
    TenantsListQuery,
    TenantsListQueryVariables
  >,
) {
  const options = { ...defaultOptions, ...baseOptions }
  return Apollo.useLazyQuery<TenantsListQuery, TenantsListQueryVariables>(
    TenantsListDocument,
    options,
  )
}
export type TenantsListQueryHookResult = ReturnType<typeof useTenantsListQuery>
export type TenantsListLazyQueryHookResult = ReturnType<
  typeof useTenantsListLazyQuery
>
export type TenantsListQueryResult = Apollo.QueryResult<
  TenantsListQuery,
  TenantsListQueryVariables
>
export const RolloutSummaryDocument = gql`
  query rolloutSummary($id: ID!) {
    rolloutSummary(id: $id) {
      id
      status
      created
      feature {
        name
      }
      rollouts {
        id
        created
        status
        environment {
          kind
        }
        events {
          id
          type
          data
          created
        }
        changeset {
          new
        }
      }
    }
  }
`

/**
 * __useRolloutSummaryQuery__
 *
 * To run a query within a React component, call `useRolloutSummaryQuery` and pass it any options that fit your needs.
 * When your component renders, `useRolloutSummaryQuery` returns an object from Apollo Client that contains loading, error, and data properties
 * you can use to render your UI.
 *
 * @param baseOptions options that will be passed into the query, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options;
 *
 * @example
 * const { data, loading, error } = useRolloutSummaryQuery({
 *   variables: {
 *      id: // value for 'id'
 *   },
 * });
 */
export function useRolloutSummaryQuery(
  baseOptions: Apollo.QueryHookOptions<
    RolloutSummaryQuery,
    RolloutSummaryQueryVariables
  >,
) {
  const options = { ...defaultOptions, ...baseOptions }
  return Apollo.useQuery<RolloutSummaryQuery, RolloutSummaryQueryVariables>(
    RolloutSummaryDocument,
    options,
  )
}
export function useRolloutSummaryLazyQuery(
  baseOptions?: Apollo.LazyQueryHookOptions<
    RolloutSummaryQuery,
    RolloutSummaryQueryVariables
  >,
) {
  const options = { ...defaultOptions, ...baseOptions }
  return Apollo.useLazyQuery<RolloutSummaryQuery, RolloutSummaryQueryVariables>(
    RolloutSummaryDocument,
    options,
  )
}
export type RolloutSummaryQueryHookResult = ReturnType<
  typeof useRolloutSummaryQuery
>
export type RolloutSummaryLazyQueryHookResult = ReturnType<
  typeof useRolloutSummaryLazyQuery
>
export type RolloutSummaryQueryResult = Apollo.QueryResult<
  RolloutSummaryQuery,
  RolloutSummaryQueryVariables
>
export const FeatureStatusDocument = gql`
  query featureStatus($envID: ID!, $feature: String!) {
    featureStatus(envID: $envID, feature: $feature) {
      environmentID
      feature
      version
      status
      configHash
      created
      lastModified
      missingDependencies {
        name
      }
    }
  }
`

/**
 * __useFeatureStatusQuery__
 *
 * To run a query within a React component, call `useFeatureStatusQuery` and pass it any options that fit your needs.
 * When your component renders, `useFeatureStatusQuery` returns an object from Apollo Client that contains loading, error, and data properties
 * you can use to render your UI.
 *
 * @param baseOptions options that will be passed into the query, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options;
 *
 * @example
 * const { data, loading, error } = useFeatureStatusQuery({
 *   variables: {
 *      envID: // value for 'envID'
 *      feature: // value for 'feature'
 *   },
 * });
 */
export function useFeatureStatusQuery(
  baseOptions: Apollo.QueryHookOptions<
    FeatureStatusQuery,
    FeatureStatusQueryVariables
  >,
) {
  const options = { ...defaultOptions, ...baseOptions }
  return Apollo.useQuery<FeatureStatusQuery, FeatureStatusQueryVariables>(
    FeatureStatusDocument,
    options,
  )
}
export function useFeatureStatusLazyQuery(
  baseOptions?: Apollo.LazyQueryHookOptions<
    FeatureStatusQuery,
    FeatureStatusQueryVariables
  >,
) {
  const options = { ...defaultOptions, ...baseOptions }
  return Apollo.useLazyQuery<FeatureStatusQuery, FeatureStatusQueryVariables>(
    FeatureStatusDocument,
    options,
  )
}
export type FeatureStatusQueryHookResult = ReturnType<
  typeof useFeatureStatusQuery
>
export type FeatureStatusLazyQueryHookResult = ReturnType<
  typeof useFeatureStatusLazyQuery
>
export type FeatureStatusQueryResult = Apollo.QueryResult<
  FeatureStatusQuery,
  FeatureStatusQueryVariables
>
export const FeatureLogsDocument = gql`
  query featureLogs($envID: ID!, $feature: String!) {
    featureStatus(envID: $envID, feature: $feature) {
      log
    }
  }
`

/**
 * __useFeatureLogsQuery__
 *
 * To run a query within a React component, call `useFeatureLogsQuery` and pass it any options that fit your needs.
 * When your component renders, `useFeatureLogsQuery` returns an object from Apollo Client that contains loading, error, and data properties
 * you can use to render your UI.
 *
 * @param baseOptions options that will be passed into the query, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options;
 *
 * @example
 * const { data, loading, error } = useFeatureLogsQuery({
 *   variables: {
 *      envID: // value for 'envID'
 *      feature: // value for 'feature'
 *   },
 * });
 */
export function useFeatureLogsQuery(
  baseOptions: Apollo.QueryHookOptions<
    FeatureLogsQuery,
    FeatureLogsQueryVariables
  >,
) {
  const options = { ...defaultOptions, ...baseOptions }
  return Apollo.useQuery<FeatureLogsQuery, FeatureLogsQueryVariables>(
    FeatureLogsDocument,
    options,
  )
}
export function useFeatureLogsLazyQuery(
  baseOptions?: Apollo.LazyQueryHookOptions<
    FeatureLogsQuery,
    FeatureLogsQueryVariables
  >,
) {
  const options = { ...defaultOptions, ...baseOptions }
  return Apollo.useLazyQuery<FeatureLogsQuery, FeatureLogsQueryVariables>(
    FeatureLogsDocument,
    options,
  )
}
export type FeatureLogsQueryHookResult = ReturnType<typeof useFeatureLogsQuery>
export type FeatureLogsLazyQueryHookResult = ReturnType<
  typeof useFeatureLogsLazyQuery
>
export type FeatureLogsQueryResult = Apollo.QueryResult<
  FeatureLogsQuery,
  FeatureLogsQueryVariables
>
export const TenantGetDocument = gql`
  query TenantGet($id: ID!) {
    tenant(id: $id) {
      id
      name
      description
      created
      lastModified
    }
  }
`

/**
 * __useTenantGetQuery__
 *
 * To run a query within a React component, call `useTenantGetQuery` and pass it any options that fit your needs.
 * When your component renders, `useTenantGetQuery` returns an object from Apollo Client that contains loading, error, and data properties
 * you can use to render your UI.
 *
 * @param baseOptions options that will be passed into the query, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options;
 *
 * @example
 * const { data, loading, error } = useTenantGetQuery({
 *   variables: {
 *      id: // value for 'id'
 *   },
 * });
 */
export function useTenantGetQuery(
  baseOptions: Apollo.QueryHookOptions<TenantGetQuery, TenantGetQueryVariables>,
) {
  const options = { ...defaultOptions, ...baseOptions }
  return Apollo.useQuery<TenantGetQuery, TenantGetQueryVariables>(
    TenantGetDocument,
    options,
  )
}
export function useTenantGetLazyQuery(
  baseOptions?: Apollo.LazyQueryHookOptions<
    TenantGetQuery,
    TenantGetQueryVariables
  >,
) {
  const options = { ...defaultOptions, ...baseOptions }
  return Apollo.useLazyQuery<TenantGetQuery, TenantGetQueryVariables>(
    TenantGetDocument,
    options,
  )
}
export type TenantGetQueryHookResult = ReturnType<typeof useTenantGetQuery>
export type TenantGetLazyQueryHookResult = ReturnType<
  typeof useTenantGetLazyQuery
>
export type TenantGetQueryResult = Apollo.QueryResult<
  TenantGetQuery,
  TenantGetQueryVariables
>
export const TenantGetByNameDocument = gql`
  query TenantGetByName($slug: String!) {
    tenant(slug: $slug) {
      id
      name
      description
      environments {
        id
        name
        warnings {
          message
        }
      }
      created
      lastModified
      warnings {
        message
        ... on FeatureWarning {
          feature {
            name
          }
          environment {
            name
          }
        }
        ... on NaisdWarning {
          environment {
            name
          }
        }
      }
    }
  }
`

/**
 * __useTenantGetByNameQuery__
 *
 * To run a query within a React component, call `useTenantGetByNameQuery` and pass it any options that fit your needs.
 * When your component renders, `useTenantGetByNameQuery` returns an object from Apollo Client that contains loading, error, and data properties
 * you can use to render your UI.
 *
 * @param baseOptions options that will be passed into the query, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options;
 *
 * @example
 * const { data, loading, error } = useTenantGetByNameQuery({
 *   variables: {
 *      slug: // value for 'slug'
 *   },
 * });
 */
export function useTenantGetByNameQuery(
  baseOptions: Apollo.QueryHookOptions<
    TenantGetByNameQuery,
    TenantGetByNameQueryVariables
  >,
) {
  const options = { ...defaultOptions, ...baseOptions }
  return Apollo.useQuery<TenantGetByNameQuery, TenantGetByNameQueryVariables>(
    TenantGetByNameDocument,
    options,
  )
}
export function useTenantGetByNameLazyQuery(
  baseOptions?: Apollo.LazyQueryHookOptions<
    TenantGetByNameQuery,
    TenantGetByNameQueryVariables
  >,
) {
  const options = { ...defaultOptions, ...baseOptions }
  return Apollo.useLazyQuery<
    TenantGetByNameQuery,
    TenantGetByNameQueryVariables
  >(TenantGetByNameDocument, options)
}
export type TenantGetByNameQueryHookResult = ReturnType<
  typeof useTenantGetByNameQuery
>
export type TenantGetByNameLazyQueryHookResult = ReturnType<
  typeof useTenantGetByNameLazyQuery
>
export type TenantGetByNameQueryResult = Apollo.QueryResult<
  TenantGetByNameQuery,
  TenantGetByNameQueryVariables
>
export const TenantsGetDocument = gql`
  query TenantsGet {
    tenants {
      id
      name
      description
      created
      lastModified
      warnings {
        message
      }
    }
    outdatedInfo {
      dependency
    }
  }
`

/**
 * __useTenantsGetQuery__
 *
 * To run a query within a React component, call `useTenantsGetQuery` and pass it any options that fit your needs.
 * When your component renders, `useTenantsGetQuery` returns an object from Apollo Client that contains loading, error, and data properties
 * you can use to render your UI.
 *
 * @param baseOptions options that will be passed into the query, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options;
 *
 * @example
 * const { data, loading, error } = useTenantsGetQuery({
 *   variables: {
 *   },
 * });
 */
export function useTenantsGetQuery(
  baseOptions?: Apollo.QueryHookOptions<
    TenantsGetQuery,
    TenantsGetQueryVariables
  >,
) {
  const options = { ...defaultOptions, ...baseOptions }
  return Apollo.useQuery<TenantsGetQuery, TenantsGetQueryVariables>(
    TenantsGetDocument,
    options,
  )
}
export function useTenantsGetLazyQuery(
  baseOptions?: Apollo.LazyQueryHookOptions<
    TenantsGetQuery,
    TenantsGetQueryVariables
  >,
) {
  const options = { ...defaultOptions, ...baseOptions }
  return Apollo.useLazyQuery<TenantsGetQuery, TenantsGetQueryVariables>(
    TenantsGetDocument,
    options,
  )
}
export type TenantsGetQueryHookResult = ReturnType<typeof useTenantsGetQuery>
export type TenantsGetLazyQueryHookResult = ReturnType<
  typeof useTenantsGetLazyQuery
>
export type TenantsGetQueryResult = Apollo.QueryResult<
  TenantsGetQuery,
  TenantsGetQueryVariables
>
export const UserInfoDocument = gql`
  query UserInfo {
    userInfo {
      email
    }
  }
`

/**
 * __useUserInfoQuery__
 *
 * To run a query within a React component, call `useUserInfoQuery` and pass it any options that fit your needs.
 * When your component renders, `useUserInfoQuery` returns an object from Apollo Client that contains loading, error, and data properties
 * you can use to render your UI.
 *
 * @param baseOptions options that will be passed into the query, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options;
 *
 * @example
 * const { data, loading, error } = useUserInfoQuery({
 *   variables: {
 *   },
 * });
 */
export function useUserInfoQuery(
  baseOptions?: Apollo.QueryHookOptions<UserInfoQuery, UserInfoQueryVariables>,
) {
  const options = { ...defaultOptions, ...baseOptions }
  return Apollo.useQuery<UserInfoQuery, UserInfoQueryVariables>(
    UserInfoDocument,
    options,
  )
}
export function useUserInfoLazyQuery(
  baseOptions?: Apollo.LazyQueryHookOptions<
    UserInfoQuery,
    UserInfoQueryVariables
  >,
) {
  const options = { ...defaultOptions, ...baseOptions }
  return Apollo.useLazyQuery<UserInfoQuery, UserInfoQueryVariables>(
    UserInfoDocument,
    options,
  )
}
export type UserInfoQueryHookResult = ReturnType<typeof useUserInfoQuery>
export type UserInfoLazyQueryHookResult = ReturnType<
  typeof useUserInfoLazyQuery
>
export type UserInfoQueryResult = Apollo.QueryResult<
  UserInfoQuery,
  UserInfoQueryVariables
>
