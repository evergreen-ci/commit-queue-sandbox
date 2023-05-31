import { Component } from 'react';

import styled from '@emotion/styled';
import Badge, { Variant } from '@leafygreen-ui/badge';
import Banner from '@leafygreen-ui/banner';
import { Tab, Tabs } from '@leafygreen-ui/tabs';
import Tooltip from '@leafygreen-ui/tooltip';

import { DataProtectionSettings } from '@packages/types/nds/backup/jobTypes';
import {
    ClusterDescription,
    ClusterType,
    EncryptionAtRestProvider,
    VersionReleaseSystem,
} from '@packages/types/nds/clusterDescription';
import { ClusterBuilderFilterInterface, DeploymentType } from '@packages/types/nds/clusterEditor';
import { ClusterOutageSimulation } from '@packages/types/nds/clusterOutageSimulation';
import { ProcessArgs } from '@packages/types/nds/ProcessArgs';
import {
    BackingCloudProvider,
    CloudProvider,
    CrossCloudProviderOptionsView,
    InstanceClass,
    InstanceSize,
    InstanceSizes,
} from '@packages/types/nds/provider';
import { Region } from '@packages/types/nds/region';
import {
    AutoScaling,
    CloudProviderOptions,
    ProviderOptions,
    ReplicationSpec,
    ReplicationSpecList,
} from '@packages/types/nds/replicationSpec';
import { EncryptionAtRest } from '@packages/types/nds/security/enterpriseSecurity';

import Accordion from '@packages/components/Accordion';
import DocsLink from '@packages/components/DocsLink';
import ExceptionAlert from '@packages/components/ExceptionAlert';
import ImagePreloader from '@packages/components/ImagePreloader';
import { SearchDeploymentSpec } from '@packages/search/src/decoupled/types';

import * as clusterEditorUtils from 'js/project/nds/common/utils/clusterEditorUtils';
import backupApi from 'js/common/services/api/nds/backupApi';
import localStorage from 'js/common/services/localStorage';
import { isAtLeast5_0 } from 'js/common/utils/mongoDbVersionHelpers';
// analytics
import analytics, { SEGMENT_EVENTS } from 'js/common/utils/segmentAnalytics';
import { RecursivePartial } from 'js/common/utils/utilTypes';
import Settings from 'js/project/common/models/Settings';
import { isNonServerlessCloudProvider } from 'js/project/nds/common/utils/cloudProviderUtils';

import * as clusterDescriptionUtils from 'js/project/nds/clusters/util/clusterDescriptionUtils';
import { NDSClusterFormContext } from 'js/project/nds/clusterEditor/NDSClusterForm/context/NDSClusterFormContext';
import NDSClusterFormAdvancedOptions from 'js/project/nds/clusterEditor/NDSClusterForm/NDSClusterFormAdvancedOptions/NDSClusterFormAdvancedOptions';
import NDSClusterFormBackup from 'js/project/nds/clusterEditor/NDSClusterForm/NDSClusterFormBackup';
import NDSClusterFormBIConnector from 'js/project/nds/clusterEditor/NDSClusterForm/NDSClusterFormBIConnector';
import NDSClusterFormBIConnectorAdvanced from 'js/project/nds/clusterEditor/NDSClusterForm/NDSClusterFormBIConnectorAdvanced';
import { ClusterState } from 'js/project/nds/clusterEditor/NDSClusterForm/NDSClusterFormController';
import NDSClusterFormEncryptionAtRestProvider from 'js/project/nds/clusterEditor/NDSClusterForm/NDSClusterFormEncryptionAtRestProvider';
import NDSClusterFormGeoOverview from 'js/project/nds/clusterEditor/NDSClusterForm/NDSClusterFormGeoOverview';
import NDSClusterFormGeoZones from 'js/project/nds/clusterEditor/NDSClusterForm/NDSClusterFormGeoZones';
import NDSClusterFormInstanceSize from 'js/project/nds/clusterEditor/NDSClusterForm/NDSClusterFormInstanceSize';
import NDSClusterFormName from 'js/project/nds/clusterEditor/NDSClusterForm/NDSClusterFormName';
import NDSClusterFormProviderButtons from 'js/project/nds/clusterEditor/NDSClusterForm/NDSClusterFormProviderButtons';
import NDSClusterFormRegionSection from 'js/project/nds/clusterEditor/NDSClusterForm/NDSClusterFormRegionSection';
import NDSClusterFormSharding from 'js/project/nds/clusterEditor/NDSClusterForm/NDSClusterFormSharding';
import NDSClusterFormTerminationProtection from 'js/project/nds/clusterEditor/NDSClusterForm/NDSClusterFormTerminationProtection';
import NDSClusterFormVersion from 'js/project/nds/clusterEditor/NDSClusterForm/NDSClusterFormVersion';
import { PROVIDER_TEXT } from 'js/project/nds/clusterEditor/NDSClusterForm/schema';
import backupUtils from 'js/project/nds/clusters/backup/utils/backupUtils';
import UseCaseBasedClusterTemplatesTooltipCard from 'js/project/nds/clusters/components/UseCaseBasedClusterTemplatesPage/UseCaseBasedClusterTemplatesTooltipCard';
import {
    AdvancedTemplates,
    AdvancedTemplatesType,
    UseCaseTemplatesType,
} from 'js/project/nds/clusters/components/UseCaseBasedClusterTemplatesPage/UseCaseBasedTemplatesData';
import {
    addClusterTemplateNodes,
    hasExistingNodes,
} from 'js/project/nds/clusters/components/UseCaseBasedClusterTemplatesPage/UseCaseBasedTemplatesUtil';
import {
    getClusterTierSecondarySubtext,
    getClusterTierSubtext,
    getCrossRegionSubtext,
    getProviderOptionsToUse,
} from 'js/project/nds/clusters/util/clusterBuilderHelpers';
import { NodeTypeFamily } from 'js/project/nds/clusters/util/clusterDescriptionUtils';
import replicationSpecListUtils from 'js/project/nds/clusters/util/replicationSpecListUtils';
import { ContainerShape } from 'js/project/nds/security/components/peering/peeringSchema';
import { BiConnectorCostEstimate } from 'js/project/nds/types/biConnectorCostEstimate';
import { BillingEstimate } from 'js/project/nds/types/billingEstimate';
import { ClusterUsageStats } from 'js/project/nds/types/ClusterUsageStats';
import { DefaultTemplates } from 'js/project/nds/types/defaultTemplate';
import Limits from 'js/project/nds/types/Limits';
import { CustomRole } from 'js/project/nds/types/role';

const ClusterTierContainer = styled.div<{ shouldDisplayFlex: boolean }>((props) => ({
    display: `${props.shouldDisplayFlex ? 'flex' : 'block'}`,
    width: `${props.shouldDisplayFlex ? '1200px' : 'inherit'}`,
}));

const ClusterTierLeftContainer = styled.div`
  width: 870px;
`;

export const ACCORDION_NAMES = {
    GLOBAL_CONFIGURATION: {
        name: 'globalConfiguration',
        headlineText: `Global Cluster Configuration`,
    },
    REGION_CONFIGURATION: { name: 'regionConfiguration' },
    CLOUD_PROVIDER: {
        name: 'cloudProvider',
        headlineText: 'Cloud Provider & Region',
    },
    INSTANCE_TIER: { name: 'instanceTier', headlineText: `Cluster Tier` },
    ADDITIONAL_SETTINGS: {
        name: 'additionalSettings',
        headlineText: 'Additional Settings',
    },
    NAME_CLUSTER: { name: 'nameCluster', headlineText: `Cluster Name` },
};

const DEFAULT_NODE_TYPE_SET = new Set<NodeTypeFamily>().add(NodeTypeFamily.BASE).add(NodeTypeFamily.ANALYTICS);

interface Props {
    setCloudProvider: (provider: CloudProvider) => void;
    setAutoScaling: (newAutoScaling: RecursivePartial<AutoScaling>, nodeTypeFamilySet: Set<NodeTypeFamily>) => void;
    setClusterFormValue: <K extends keyof ClusterState>(key: K, value: any, callback?: () => void) => void;
    setInstanceSize: (
        newBaseInstanceSize: InstanceSize,
        nodeTypeFamilySet: Set<NodeTypeFamily>,
        callback?: (updatedFormValues: ClusterDescription) => unknown
    ) => void;
    formValues: ClusterDescription;
    defaultTemplates: DefaultTemplates;
    providerOptions: ProviderOptions;
    crossCloudProviderOptions: CrossCloudProviderOptionsView;
    error: null | { message: string; errorCode: string };
    instanceClass: InstanceClass;
    analyticsInstanceClass: InstanceClass;
    processArgs: ProcessArgs;
    originalProcessArgs: ProcessArgs;
    isEdit: boolean;
    hasFreeCluster: boolean;
    isTenantUpgrade: boolean;
    encryptionAtRest: EncryptionAtRest;
    isLDAPEnabled: boolean;
    isPrivateIPModeEnabled: boolean;
    useCNRegionsOnly: boolean;
    originalCluster: ClusterDescription;
    settingsModel: typeof Settings;
    billingEstimate: BillingEstimate;
    biConnectorCostEstimate: BiConnectorCostEstimate;
    clusterUsageStats: ClusterUsageStats;
    cloudContainers: Array<ContainerShape>;
    customRoles: Array<CustomRole>;
    clusterBuilderFilterInterface: ClusterBuilderFilterInterface;
    isNameInUseByClusterWithBackupSnapshots: boolean;
    isNameValid: boolean;
    isNameUnique: boolean;
    isNamePrefixUnique: boolean;
    isNameEndWithHyphen: boolean;
    isAutoIndexingEligible: boolean;
    isInstanceSizeVisible: (string) => boolean;
    deploymentType: DeploymentType;
    setDeploymentType: (deploymentType: DeploymentType) => void;
    openSection: null | keyof typeof ACCORDION_NAMES;
    groupLimits: Limits;
    hasUnsetOplogMinRetentionHours: boolean;
    template?: UseCaseTemplatesType;
    isInUseCaseBasedClusterTemplates: boolean;
    isDiskSizeLargerThanStandardMax: boolean;
    clusterOutageSimulation?: ClusterOutageSimulation;
    updateCrossCloudProviderOptions: (replicationSpecList: ReplicationSpecList) => Promise<void>;
    hasDecoupledSearchFeatureEnabled: boolean;
    searchNodesEnabled: boolean;
    searchConsiderationConfirmed: boolean;
    searchDeploymentSpec: SearchDeploymentSpec;
}

interface State {
    newFormSelectedReplicationSpecId: string;
    openAccordions: Partial<Record<keyof typeof ACCORDION_NAMES, boolean>>;
    highlightAccordions: Partial<Record<string, boolean>>;
    selectedClusterTierTab: number;
    showSqlInterfaceBanner: boolean;
    dataProtectionSettings: DataProtectionSettings | null;
}

class NDSClusterForm extends Component<Props, State> {
    static contextType = NDSClusterFormContext;

    static getDerivedStateFromProps(nextProps, prevState) {
        const { formValues } = nextProps;

        const newState: Partial<State> = {};

        if (
            formValues.replicationSpecList &&
            replicationSpecListUtils.getReplicationSpecIndexFromId(
                formValues.replicationSpecList,
                prevState.newFormSelectedReplicationSpecId
            ) === -1
        ) {
            newState.newFormSelectedReplicationSpecId = formValues.replicationSpecList[0].id;
        }

        return newState;
    }

    state: State = {
        // if global configuration filter is chosen, the GeoOverview accordion should default to open
        openAccordions: {
            [ACCORDION_NAMES.GLOBAL_CONFIGURATION.name]:
            this.props.clusterBuilderFilterInterface.providerTemplateKey === 'geoSharded3Zone',
            [ACCORDION_NAMES.INSTANCE_TIER.name]:
            this.props.isTenantUpgrade || this.props.openSection === ACCORDION_NAMES.INSTANCE_TIER.name,
            [ACCORDION_NAMES.ADDITIONAL_SETTINGS.name]: this.props.openSection === ACCORDION_NAMES.ADDITIONAL_SETTINGS.name,
        },
        highlightAccordions: {},
        // the only case where it would be undefined is in tests that don't have fixtures populated yet -- TODO CLOUDP-72869
        newFormSelectedReplicationSpecId: this.props.formValues.replicationSpecList[0].id,
        selectedClusterTierTab: 0,
        showSqlInterfaceBanner: localStorage.getItem(this.getDismissKey()) !== 'dismissed',
        dataProtectionSettings: null,
    };

    getDismissKey() {
        const { settingsModel } = this.props;
        return `MMS.ClusterForm.AtlasSqlInterfaceBanner.${settingsModel.getCurrentGroupId()}`;
    }

    componentDidMount() {
        const {
            isEdit,
            formValues,
            settingsModel,
            template: selectedClusterTemplate,
            isInUseCaseBasedClusterTemplates,
        } = this.props;
        const isGeoZoned = formValues.clusterType === ClusterType.GEOSHARDED;

        if (isEdit && isGeoZoned) {
            this.setAccordionOpened(ACCORDION_NAMES.GLOBAL_CONFIGURATION.name, true);
        } else if (!isEdit) {
            // when creating a new cluster, trigger the animation of the first accordion opening
            // so that a new user can understand the accordion behavior

            setTimeout(() => {
                this.setAccordionOpened(ACCORDION_NAMES.CLOUD_PROVIDER.name, true);
            }, 0);
        }

        const isBaseClusterTemplate = selectedClusterTemplate && selectedClusterTemplate in InstanceSizes;
        const isAdvancedClusterTemplate =
            selectedClusterTemplate && Object.keys(AdvancedTemplates).includes(selectedClusterTemplate);
        const currentInstanceSizeName = replicationSpecListUtils.getFirstInstanceSize(formValues.replicationSpecList);
        const isFreeOrSharedTierCluster = clusterDescriptionUtils.isFreeOrSharedTierCluster(currentInstanceSizeName);

        // For use case based cluster templates experiment
        if (isInUseCaseBasedClusterTemplates && selectedClusterTemplate) {
            if (isBaseClusterTemplate && selectedClusterTemplate in InstanceSizes) {
                // Update the instance size
                this.setInstanceSize(selectedClusterTemplate as InstanceSize, DEFAULT_NODE_TYPE_SET);
            } else if (isAdvancedClusterTemplate && isFreeOrSharedTierCluster) {
                // Check if the current cluster is free/shared, if so upgrade to M10 and then add nodes
                this.setInstanceSize(InstanceSizes.M10, DEFAULT_NODE_TYPE_SET, (updatedFormValues) => {
                    addClusterTemplateNodes({
                        selectedClusterTemplate: selectedClusterTemplate!,
                        formValues: updatedFormValues,
                        providerOptions: this.props.providerOptions,
                        defaultReplicationSpecRecord: this.getMemoizedBackingProviderDefaultOneZoneReplicationSpec('replicaSet'),
                        defaultTemplates: this.props.defaultTemplates,
                        updateReplicationSpecList: this.updateReplicationSpecList,
                        updateCrossCloudProviderOptions: this.props.updateCrossCloudProviderOptions,
                    });
                });
            } else if (isAdvancedClusterTemplate) {
                // Just add the nodes, no need to update instance size
                addClusterTemplateNodes({
                    selectedClusterTemplate,
                    formValues: this.props.formValues,
                    providerOptions: this.props.providerOptions,
                    defaultReplicationSpecRecord: this.getMemoizedBackingProviderDefaultOneZoneReplicationSpec('replicaSet'),
                    defaultTemplates: this.props.defaultTemplates,
                    updateReplicationSpecList: this.updateReplicationSpecList,
                    updateCrossCloudProviderOptions: this.props.updateCrossCloudProviderOptions,
                });
            }
        } else {
            // Set context for default cluster size
            this.context.setDesiredInstanceSize(currentInstanceSizeName);
            const currentAnalyticsInstanceSizeName = replicationSpecListUtils.getFirstAnalyticsInstanceSize(
                formValues.replicationSpecList
            );
            this.context.setDesiredAnalyticsInstanceSize(currentAnalyticsInstanceSizeName);
        }

        backupApi.getDataProtectionSettings(settingsModel.getCurrentGroupId()).then((dataProtectionSettings) => {
            if (dataProtectionSettings) {
                this.setState({
                    dataProtectionSettings: dataProtectionSettings as DataProtectionSettings,
                });
            }
        });
    }

    componentDidUpdate(prevProps) {
        if (!prevProps.error && this.props.error) {
            window.scrollTo(0, 0);
        }
    }

    getBackupSubtext = (diskBackupAllowed) => {
        if (diskBackupAllowed) {
            if (this.props.formValues.diskBackupEnabled) {
                return 'Cloud Backup';
            }
            if (this.props.formValues.backupEnabled) {
                return 'Legacy Backup';
            }
        }

        return '';
    };

    getAdditionalSettingsSecondaryText() {
        const { formValues, originalCluster, crossCloudProviderOptions } = this.props;
        const hasAnyBackupEnabled =
            formValues.backupEnabled || formValues.diskBackupEnabled || !!formValues.tenantBackupEnabled;
        const backupText = hasAnyBackupEnabled ? 'Backup' : 'No Backup';
        const totalShards = replicationSpecListUtils.getTotalShards(formValues.replicationSpecList);
        const shardingText = formValues.clusterType === 'SHARDED' ? `, ${totalShards} Shards` : '';
        const biConnectorText = formValues.biConnector.enabled ? ', BI Connector' : '';
        const encryptionAtRestProvider =
            formValues.encryptionAtRestProvider !== EncryptionAtRestProvider.NONE ? ', Encryption at Rest' : '';
        const selectedMongoDBMajorVersion =
            formValues.mongoDBMajorVersion === '4.3' ? '4.4' : formValues.mongoDBMajorVersion;
        const mongoDBVersionToShow = clusterDescriptionUtils.getCurrentClusterVersionToShow(
            selectedMongoDBMajorVersion,
            formValues.versionReleaseSystem === VersionReleaseSystem.CONTINUOUS,
            crossCloudProviderOptions.defaultCDMongoDBFCV,
            originalCluster.fixedMongoDBFCV
        );

        return `
      ${
            originalCluster.versionReleaseSystem === 'CONTINUOUS' ? 'Latest Release' : `MongoDB ${mongoDBVersionToShow}`
        }, ${backupText}${shardingText}${biConnectorText}${encryptionAtRestProvider}`;
    }

    onDismissSqlInterfaceBanner = () => {
        this.setState({ showSqlInterfaceBanner: false });
        localStorage.setItem(this.getDismissKey(), 'dismissed');
    };

    onClickSqlInterfaceDocs = () => {
        analytics.track(SEGMENT_EVENTS.UX_ACTION_PERFORMED, {
            context: 'Cluster Builder',
            action: 'Clicked Atlas SQL Interface Docs',
        });
    };

    setAccordionOpened = (name, isOpen, callback = () => {}) => {
        const open = this.state.openAccordions;
        const highlight = this.state.highlightAccordions;
        highlight[name] = isOpen;
        open[name] = isOpen;

        this.setState(
            {
                openAccordions: open,
                highlightAccordions: highlight,
            },
            callback
        );
    };

    setInstanceSize = (
        instanceSize: InstanceSize,
        nodeTypeFamilySet: Set<NodeTypeFamily>,
        callback?: (updatedFormValues: ClusterDescription) => unknown
    ) => {
        const { setInstanceSize } = this.props;
        setInstanceSize(instanceSize, nodeTypeFamilySet, callback);
    };

    setNewFormSelectedReplicationSpecId = (id) => {
        if (id !== this.state.newFormSelectedReplicationSpecId) {
            this.setState({ newFormSelectedReplicationSpecId: id });
        }
    };

    toggleActive =
        (toggle, callback = () => {}) =>
            () => {
                const open = this.state.openAccordions;

                // Leaving these analytics events referencing "databases" as "cluster" for backwards compatibility
                if (open[toggle.name]) {
                    if (toggle.headlineText) {
                        analytics.track(SEGMENT_EVENTS.UX_ACTION_PERFORMED, {
                            context: 'Cluster Builder',
                            action: 'Accordion Closed',
                            value: toggle.headlineText,
                            pathfinder_filter: `${toggle.headlineText} Accordion Closed`,
                        });
                    }
                    this.setAccordionOpened(toggle.name, false, callback);
                } else {
                    if (toggle.headlineText) {
                        analytics.track(SEGMENT_EVENTS.UX_ACTION_PERFORMED, {
                            context: 'Cluster Builder',
                            action: 'Accordion Opened',
                            value: toggle.headlineText,
                            pathfinder_filter: `${toggle.headlineText} Accordion Opened`,
                        });
                    }
                    this.setAccordionOpened(toggle.name, true, callback);
                }
            };

    updateReplicationSpecList = (newSpec, callback?) => {
        const { formValues, setClusterFormValue } = this.props;

        let newReplicationSpecList = replicationSpecListUtils.updateReplicationSpecInList(
            formValues.replicationSpecList,
            newSpec
        );

        if (replicationSpecListUtils.getTotalAnalyticsNodes(newSpec) === 0) {
            const baseInstanceSizeName = replicationSpecListUtils.getFirstInstanceSize(newReplicationSpecList);
            newReplicationSpecList = replicationSpecListUtils.updateAnalyticsInstanceSize(
                baseInstanceSizeName,
                newReplicationSpecList
            );
        }

        setClusterFormValue('replicationSpecList', newReplicationSpecList, callback);
    };

    ensureProvisionedIOPSVolumeType = () => {
        const { formValues, setClusterFormValue } = this.props;

        const currentVolumeType = replicationSpecListUtils.getFirstAWSVolumeType(formValues.replicationSpecList);

        const newReplicationSpecList = replicationSpecListUtils.ensureProvisionedIOPSVolumeType(
            formValues.replicationSpecList
        );

        const newVolumeType = replicationSpecListUtils.getFirstAWSVolumeType(newReplicationSpecList);

        if (newVolumeType !== currentVolumeType) {
            setClusterFormValue('ebsVolumeType', newVolumeType);
            setClusterFormValue('replicationSpecList', newReplicationSpecList);
        }
    };

    getPreferredRegionReplicationSpecs = (replicationSpecList): Record<string, string> => {
        const mapping = {};
        replicationSpecList.forEach((replicationSpec) => {
            const replicationId = replicationSpec.id;
            mapping[replicationId] = replicationSpecListUtils.getPreferredRegion(replicationSpec).regionName;
        });
        return mapping;
    };

    hasPITHighPriorityRegionChanged = () => {
        const { originalCluster, formValues } = this.props;
        const preferredRegionOrig = this.getPreferredRegionReplicationSpecs(originalCluster.replicationSpecList);
        const preferredRegionEdit = this.getPreferredRegionReplicationSpecs(formValues.replicationSpecList);
        if (!originalCluster.pitEnabled) {
            return false;
        }
        // check if the two are equal
        if (preferredRegionOrig.size !== preferredRegionEdit.size) {
            return true;
        }
        for (const [id, highestPriorityRegionOrig] of Object.entries(preferredRegionOrig)) {
            const highestPriorityRegionEdit = preferredRegionEdit[id];
            if (highestPriorityRegionEdit !== highestPriorityRegionOrig) {
                return true;
            }
        }
        return false;
    };

    hasPITBeenToggledOff = () => {
        const { originalCluster, formValues } = this.props;
        return originalCluster.pitEnabled && !formValues.pitEnabled;
    };

    hasPITBeenToggledOn = () => {
        const { originalCluster, formValues } = this.props;
        return !originalCluster.pitEnabled && formValues.pitEnabled;
    };

    isShardingSupported = (isTenant: boolean, isInstanceSizeSupportSharding: boolean) => {
        return !isTenant && isInstanceSizeSupportSharding;
    };

    getProviderRegions = (
        providerOptionsToUse: CloudProviderOptions,
        currentBackingProviders: Array<BackingCloudProvider>
    ): Array<Region> => {
        const { providerOptions, deploymentType } = this.props;

        if (deploymentType !== DeploymentType.SHARED) {
            return providerOptionsToUse.regions;
        }
        const freeInstances = providerOptions.FREE.instanceSizes;
        const availableFreeInstanceRegions = Object.keys(freeInstances).flatMap((k) => freeInstances[k].availableRegions);
        const availableFreeInstanceRegionNamesForCurrentProvider = availableFreeInstanceRegions
            .filter((region) => region.providerName === currentBackingProviders[0])
            .map((availableRegion) => availableRegion.regionName);

        return providerOptionsToUse.regions.filter((region) =>
            availableFreeInstanceRegionNamesForCurrentProvider.includes(region.key)
        );
    };

    setSelected = (index: number): void => {
        this.setState({ selectedClusterTierTab: index });
    };

    renderGlobalConfigurationSection = (
        backingProviderDefaultOneZoneReplicationSpec: Record<BackingCloudProvider, ReplicationSpec>
    ) => {
        const {
            setCloudProvider,
            setClusterFormValue,
            formValues,
            defaultTemplates,
            providerOptions,
            crossCloudProviderOptions,
            isEdit,
            settingsModel,
            originalCluster,
            isPrivateIPModeEnabled,
            useCNRegionsOnly,
            cloudContainers,
            clusterBuilderFilterInterface,
            groupLimits,
            isDiskSizeLargerThanStandardMax,
            clusterOutageSimulation,
        } = this.props;
        const { openAccordions, highlightAccordions, newFormSelectedReplicationSpecId } = this.state;

        const replicationSpecIndex = replicationSpecListUtils.getReplicationSpecIndexFromId(
            formValues.replicationSpecList,
            newFormSelectedReplicationSpecId
        );

        const selectedReplicationSpec = formValues.replicationSpecList[replicationSpecIndex];

        const backingCloudProviders = replicationSpecListUtils.getBackingCloudProviders(formValues.replicationSpecList);

        const originalInstanceSizeName = replicationSpecListUtils.getFirstInstanceSize(originalCluster.replicationSpecList);
        const instanceSizeName = replicationSpecListUtils.getFirstInstanceSize(formValues.replicationSpecList);

        const geoText = formValues.clusterType === ClusterType.GEOSHARDED ? 'Global Writes Enabled' : '';

        // for this value, FREE provider is mapped to the backing provider
        const originalBackingProviders = replicationSpecListUtils.getBackingCloudProviders(
            originalCluster.replicationSpecList
        );
        const isGeoZoned = formValues.clusterType === ClusterType.GEOSHARDED;
        const minShardingInstanceSize = crossCloudProviderOptions.minShardingInstanceSize;

        const isValid = replicationSpecListUtils.isValid(formValues.replicationSpecList, formValues.clusterType);

        const numReplicationSpecs = formValues.replicationSpecList.length;

        return (
            <>
                <Accordion
                    ref={(ref) => {
                        this[ACCORDION_NAMES.GLOBAL_CONFIGURATION.name] = ref;
                    }}
                    headlineText={ACCORDION_NAMES.GLOBAL_CONFIGURATION.headlineText}
                    secondaryText={geoText}
                    secondarySubText={
                        isGeoZoned
                            ? `${backingCloudProviders.map((p) => PROVIDER_TEXT[p]).join(', ')}, ${numReplicationSpecs} Zone${
                                formValues.replicationSpecList.length > 1 ? 's' : ''
                            }`
                            : ''
                    }
                    highlightSecondaryText={highlightAccordions[ACCORDION_NAMES.GLOBAL_CONFIGURATION.name]}
                    errorSecondaryText={!isValid}
                    active={!!openAccordions[ACCORDION_NAMES.GLOBAL_CONFIGURATION.name]}
                    onHeadlineClick={this.toggleActive(ACCORDION_NAMES.GLOBAL_CONFIGURATION)}
                    animate
                    colorCode={isGeoZoned ? clusterEditorUtils.getAccordionColor(replicationSpecIndex) : undefined}
                    hasSummary={
                        openAccordions[ACCORDION_NAMES.GLOBAL_CONFIGURATION.name] &&
                        formValues.clusterType === ClusterType.GEOSHARDED
                    }
                >
                    <NDSClusterFormGeoOverview
                        setCloudProvider={setCloudProvider}
                        setClusterFormValue={setClusterFormValue}
                        defaultTemplates={defaultTemplates}
                        isEdit={isEdit}
                        clusterType={formValues.clusterType}
                        originalClusterType={originalCluster.clusterType}
                        originalInstanceSize={originalInstanceSizeName}
                        currentInstanceSize={instanceSizeName}
                        providers={backingCloudProviders}
                        originalProviders={originalBackingProviders}
                        groupId={settingsModel.getCurrentGroupId()}
                        docsUrl={settingsModel.getDocsUrl()}
                        replicationSpecList={formValues.replicationSpecList}
                        geoSharding={formValues.geoSharding}
                        // ensure region configuration accordion is open when editing geosharding
                        onShardingEnabled={() => this.setAccordionOpened(ACCORDION_NAMES.REGION_CONFIGURATION.name, true)}
                        isPrivateIPModeEnabled={isPrivateIPModeEnabled}
                        minShardingInstanceSize={minShardingInstanceSize}
                        cloudContainers={cloudContainers}
                        showOverlayTemplates={clusterBuilderFilterInterface.providerTemplateKey !== 'geoSharded3Zone'}
                        providerOptions={providerOptions}
                        isNdsGovEnabled={settingsModel.isNdsGovEnabled()}
                        useCNRegionsOnly={useCNRegionsOnly}
                        backingProviderDefaultOneZoneReplicationSpec={backingProviderDefaultOneZoneReplicationSpec}
                        providerRegions={providerOptions[backingCloudProviders[0]].regions}
                        diskSizeGB={formValues.diskSizeGB}
                        isDiskSizeLargerThanStandardMax={isDiskSizeLargerThanStandardMax}
                    />
                </Accordion>
                {isGeoZoned && (
                    <NDSClusterFormGeoZones
                        providerOptions={providerOptions}
                        providers={backingCloudProviders}
                        replicationSpec={selectedReplicationSpec}
                        replicationSpecList={formValues.replicationSpecList}
                        originalReplicationSpecList={originalCluster.replicationSpecList}
                        backingProviderDefaultOneZoneReplicationSpec={backingProviderDefaultOneZoneReplicationSpec}
                        highlightSecondaryText={!!highlightAccordions.regionConfiguration}
                        onAccordionHeadlineClick={this.toggleActive(ACCORDION_NAMES.REGION_CONFIGURATION)}
                        providerRegions={providerOptions[backingCloudProviders[0]].regions}
                        isAccordionActive={!!openAccordions[ACCORDION_NAMES.REGION_CONFIGURATION.name]}
                        setClusterFormValue={setClusterFormValue}
                        updateReplicationSpecList={this.updateReplicationSpecList}
                        setNewFormSelectedReplicationSpecId={this.setNewFormSelectedReplicationSpecId}
                        isEdit={isEdit}
                        clusterType={formValues.clusterType}
                        isPrivateIPModeEnabled={isPrivateIPModeEnabled}
                        hasPITHighestPriorityRegionChanged={this.hasPITHighPriorityRegionChanged()}
                        isNDSGovEnabled={settingsModel.isNdsGovEnabled()}
                        useCNRegionsOnly={useCNRegionsOnly}
                        maxZones={groupLimits.maxZonesPerGeoCluster}
                        ensureProvisionedIOPSVolumeType={this.ensureProvisionedIOPSVolumeType}
                        diskSizeGB={formValues.diskSizeGB}
                        clusterOutageSimulation={clusterOutageSimulation}
                    />
                )}
            </>
        );
    };

    backingProviderDefaultOneZoneReplicationSpecMemoState: {
        backingProviderDefaultOneZoneReplicationSpec?: Record<BackingCloudProvider, ReplicationSpec>;
        defaultTemplates?: DefaultTemplates;
        defaultTemplateKey?: string;
    } = {};

    getMemoizedBackingProviderDefaultOneZoneReplicationSpec(
        defaultTemplateKey: string
    ): Record<BackingCloudProvider, ReplicationSpec> {
        const defaultTemplates = this.props.defaultTemplates;
        if (
            !this.backingProviderDefaultOneZoneReplicationSpecMemoState.backingProviderDefaultOneZoneReplicationSpec ||
            this.backingProviderDefaultOneZoneReplicationSpecMemoState.defaultTemplates !== defaultTemplates ||
            this.backingProviderDefaultOneZoneReplicationSpecMemoState.defaultTemplateKey !== defaultTemplateKey
        ) {
            const backingProviderDefaultOneZoneReplicationSpec = {
                AWS: defaultTemplates.AWS[defaultTemplateKey].replicationSpecList[0],
                GCP: defaultTemplates.GCP[defaultTemplateKey].replicationSpecList[0],
                AZURE: defaultTemplates.AZURE[defaultTemplateKey].replicationSpecList[0],
            };

            this.backingProviderDefaultOneZoneReplicationSpecMemoState = {
                backingProviderDefaultOneZoneReplicationSpec,
                defaultTemplates,
                defaultTemplateKey,
            };

            return backingProviderDefaultOneZoneReplicationSpec;
        }

        return this.backingProviderDefaultOneZoneReplicationSpecMemoState.backingProviderDefaultOneZoneReplicationSpec;
    }

    render() {
        const {
            setCloudProvider,
            setAutoScaling,
            setClusterFormValue,
            setInstanceSize,
            formValues,
            defaultTemplates,
            providerOptions,
            crossCloudProviderOptions,
            error,
            isEdit,
            processArgs,
            originalProcessArgs,
            settingsModel,
            hasFreeCluster,
            originalCluster,
            isTenantUpgrade,
            isLDAPEnabled,
            billingEstimate,
            biConnectorCostEstimate,
            encryptionAtRest,
            isPrivateIPModeEnabled,
            useCNRegionsOnly,
            clusterUsageStats,
            instanceClass,
            analyticsInstanceClass,
            cloudContainers,
            customRoles,
            clusterBuilderFilterInterface,
            isNameInUseByClusterWithBackupSnapshots,
            isNameUnique,
            isNameValid,
            isNamePrefixUnique,
            isNameEndWithHyphen,
            isAutoIndexingEligible,
            isInstanceSizeVisible,
            deploymentType,
            setDeploymentType,
            hasUnsetOplogMinRetentionHours,
            template,
            isInUseCaseBasedClusterTemplates,
            isDiskSizeLargerThanStandardMax,
            clusterOutageSimulation,
            hasDecoupledSearchFeatureEnabled,
            searchNodesEnabled,
            searchConsiderationConfirmed,
            searchDeploymentSpec,
        } = this.props;

        const {
            openAccordions,
            highlightAccordions,
            newFormSelectedReplicationSpecId,
            selectedClusterTierTab,
            showSqlInterfaceBanner,
        } = this.state;

        const currentProviders = replicationSpecListUtils
            .getCloudProviders(formValues.replicationSpecList)
            .filter(isNonServerlessCloudProvider);

        const currentBackingProviders = replicationSpecListUtils.getBackingCloudProviders(formValues.replicationSpecList);

        const providerOptionsToUse = getProviderOptionsToUse(
            formValues.replicationSpecList,
            providerOptions,
            crossCloudProviderOptions
        );
        const originalProviderOptionsToUse = getProviderOptionsToUse(
            originalCluster.replicationSpecList,
            providerOptions,
            crossCloudProviderOptions
        );

        const currentInstanceSizeName = replicationSpecListUtils.getFirstInstanceSize(formValues.replicationSpecList);
        const currentInstanceSize = providerOptionsToUse.instanceSizes[currentInstanceSizeName]!;
        const currentAnalyticsInstanceSizeName = replicationSpecListUtils.getFirstAnalyticsInstanceSize(
            formValues.replicationSpecList
        );

        const shouldShowReplicationLagWarning = replicationSpecListUtils.isAnalyticsInstanceTwoOrMoreSizesBelowBaseInstance(
            formValues.replicationSpecList,
            crossCloudProviderOptions
        );

        const originalInstanceSizeName = replicationSpecListUtils.getFirstInstanceSize(originalCluster.replicationSpecList);
        const originalAnalyticsInstanceSizeName = replicationSpecListUtils.getFirstAnalyticsInstanceSize(
            originalCluster.replicationSpecList
        );

        const replicationSpecIndex = replicationSpecListUtils.getReplicationSpecIndexFromId(
            formValues.replicationSpecList,
            newFormSelectedReplicationSpecId
        );

        const originalVersion = originalCluster.mongoDBMajorVersion;
        const selectedVersion = formValues.mongoDBMajorVersion === '4.3' ? '4.4' : formValues.mongoDBMajorVersion;
        const selectedReplicationSpec = formValues.replicationSpecList[replicationSpecIndex];
        const originalClusterPreferredCpuArch = replicationSpecListUtils.getFirstProviderBaseHardwareSpec(
            originalCluster.replicationSpecList,
            CloudProvider.AWS
        )?.preferredCpuArchitecture;

        const docsUrl = settingsModel.getDocsUrl();
        const diskBackupAllowed = clusterDescriptionUtils.isDiskBackupAllowed(currentProviders);
        const isTenant = clusterDescriptionUtils.isFreeOrSharedTierCluster(currentInstanceSizeName);
        const isFree = clusterDescriptionUtils.isFreeCluster(currentInstanceSizeName);
        const isCrossRegionEnabled = replicationSpecListUtils.isCrossRegionEnabled(selectedReplicationSpec);
        const isClusterEncryptionAtRestEnabled = formValues.encryptionAtRestProvider !== EncryptionAtRestProvider.NONE;
        const hasMultipleRegions = replicationSpecListUtils.isCrossRegionEnabled(selectedReplicationSpec);
        const preferredRegion = replicationSpecListUtils.getPreferredRegion(selectedReplicationSpec);
        const isAwsGravitonEnabled = settingsModel.hasProjectFeature('AWS_GRAVITON');
        const { instanceSizes } = providerOptionsToUse;
        const originalInstanceSizes = originalProviderOptionsToUse.instanceSizes;
        const shardingEnabled = this.isShardingSupported(isTenant, currentInstanceSize.supportsSharding);
        const autoScaling = replicationSpecListUtils.getFirstAutoScaling(formValues.replicationSpecList);
        const analyticsAutoScaling =
            replicationSpecListUtils.getFirstAnalyticsAutoScalingOrBaseFallbackOrDisabledComputeAutoScaling(
                formValues.replicationSpecList
            );
        const { compute } = autoScaling;
        const minComputeInstanceSizeSupportsSharding =
            !compute.scaleDownEnabled ||
            (compute.minInstanceSize &&
                instanceSizes[compute.minInstanceSize] &&
                instanceSizes[compute.minInstanceSize].supportsSharding);

        // for this value, FREE provider is mapped to the backing provider
        const originalBackingProviders = replicationSpecListUtils.getBackingCloudProviders(
            originalCluster.replicationSpecList
        );
        // for this value, FREE is not mapped to the backing provider
        const isGeoZoned = formValues.clusterType === ClusterType.GEOSHARDED;
        const isOriginalClusterSharded = originalCluster.clusterType === ClusterType.SHARDED;
        const isOriginalClusterSyncedWithNonOptimizedPrivateEndpoint = originalCluster.privateLinkSrvAddresses
            ? Object.keys(originalCluster.privateLinkSrvAddresses).length > 0
            : false;
        const originalInstanceSize = originalInstanceSizes[originalInstanceSizeName]!;
        const originalEncryptionAtRestProvider = originalCluster.encryptionAtRestProvider;
        const minShardingInstanceSize = providerOptions[currentBackingProviders[0]].minShardingInstanceSize;

        const isNVMe = currentInstanceSize.isNVMe;
        const useInstanceSizeMaxStorage = isEdit && originalCluster.diskSizeGB > originalInstanceSize.maxAllowedStorageGB;
        const onlyShowDiskBackup =
            currentProviders.length > 1 ||
            currentProviders[0] === 'AWS' ||
            ((currentProviders[0] === 'GCP' || currentProviders[0] === 'AZURE') &&
                settingsModel.hasProjectFeature('CPS_GCP_AND_AZURE_NEW_CLUSTERS_ONLY_CPS'));

        const hasAnalyticsNodes = clusterDescriptionUtils.getTotalAnalyticsNodes(formValues.replicationSpecList) > 0;
        const defaultTemplateKey = currentInstanceSizeName === 'M0' ? 'm0' : 'replicaSet';
        const customRoleVersions = new Set(
            customRoles.map((role) => role.minimumMongoVersion).filter((role) => role !== null)
        );
        const clusterTierHeader = (
            <div>
                <Tooltip
                    align="top"
                    justify="start"
                    trigger={<span className="accordion-headline-text underline-dotted">Cluster</span>}
                    triggerEvent="hover"
                    adjustOnMutation
                    usePortal={false}
                    className="tooltip-content-is-cluster-form"
                >
                    An Atlas-managed MongoDB deployment. Clusters can be either a replica set or a sharded cluster.
                </Tooltip>{' '}
                <span className="accordion-headline-text"> Tier</span>
            </div>
        );

        const analyticsTierHeader = (
            <span>
        Analytics Tier
        <span css={{ paddingLeft: 5 }}>
          <Badge variant="blue">New!</Badge>
        </span>
      </span>
        );

        const possibleMongoDBMajorVersions = clusterEditorUtils.getPossibleMongoDBMajorVersionsForCluster(
            providerOptionsToUse,
            currentProviders,
            currentInstanceSizeName,
            formValues.replicationSpecList
        );

        const isValid = replicationSpecListUtils.isValid(formValues.replicationSpecList, formValues.clusterType);

        const isNotSharedOrVersionSelectionAvailable =
            deploymentType !== DeploymentType.SHARED || possibleMongoDBMajorVersions.filter((v) => !v.deprecated).length > 1;

        const backingProviderDefaultOneZoneReplicationSpec =
            this.getMemoizedBackingProviderDefaultOneZoneReplicationSpec(defaultTemplateKey);

        const isClusterAWSOnly =
            currentBackingProviders.length === 1 && currentBackingProviders.includes(BackingCloudProvider.AWS);

        const updatedClusterWithPrivateEndpointSupportsOptimizedConnectionString =
            isEdit &&
            isOriginalClusterSyncedWithNonOptimizedPrivateEndpoint &&
            isClusterAWSOnly &&
            isAtLeast5_0(formValues.mongoDBMajorVersion) &&
            formValues.clusterType !== ClusterType.REPLICASET;
        const areInstanceClassesAsymmetric = instanceClass !== analyticsInstanceClass;

        // UseCaseBasedClusterTemplates experiment
        const hasSelectedAdvancedTemplate =
            isInUseCaseBasedClusterTemplates && isEdit && Object.keys(AdvancedTemplates).some((v) => v === template);
        const shouldTurnOnAdvancedToggle = hasSelectedAdvancedTemplate && !hasExistingNodes(formValues.replicationSpecList);

        const { dataProtectionSettings } = this.state;

        return (
            <div className="nds-cluster-form">
                <ImagePreloader sources={['/static/images/threezones.png', '/static/images/sixzones.png']} />
                {!settingsModel.isNdsGovEnabled() &&
                    !useCNRegionsOnly &&
                    clusterBuilderFilterInterface.isGlobalConfigurationVisible &&
                    deploymentType === DeploymentType.DEDICATED &&
                    this.renderGlobalConfigurationSection(backingProviderDefaultOneZoneReplicationSpec)}
                <ClusterTierContainer shouldDisplayFlex={hasSelectedAdvancedTemplate}>
                    <ClusterTierLeftContainer>
                        {!isGeoZoned && (
                            <Accordion
                                ref={(ref) => {
                                    this[ACCORDION_NAMES.CLOUD_PROVIDER.name] = ref;
                                }}
                                headlineText={ACCORDION_NAMES.CLOUD_PROVIDER.headlineText}
                                secondaryText={
                                    hasMultipleRegions
                                        ? `${currentBackingProviders.map((provider) => PROVIDER_TEXT[provider]).join(', ')}`
                                        : `${PROVIDER_TEXT[currentBackingProviders[0]]},
                            ${preferredRegion.regionView.location} (${preferredRegion.regionView.name})`
                                }
                                secondarySubText={
                                    isCrossRegionEnabled
                                        ? getCrossRegionSubtext(
                                            this.props.formValues.replicationSpecList,
                                            this.state.newFormSelectedReplicationSpecId
                                        )
                                        : ''
                                }
                                onHeadlineClick={this.toggleActive(ACCORDION_NAMES.CLOUD_PROVIDER)}
                                active={!!openAccordions[ACCORDION_NAMES.CLOUD_PROVIDER.name]}
                                highlightSecondaryText={highlightAccordions[ACCORDION_NAMES.CLOUD_PROVIDER.name]}
                                errorSecondaryText={!isValid}
                                animate
                            >
                                <NDSClusterFormProviderButtons
                                    setCloudProvider={setCloudProvider}
                                    originalProviders={originalBackingProviders}
                                    isTenantUpgrade={isTenantUpgrade}
                                    providers={currentBackingProviders}
                                    isPrivateIPModeEnabled={isPrivateIPModeEnabled}
                                    instanceSize={currentInstanceSizeName}
                                    cloudContainers={cloudContainers}
                                    isNdsGovEnabled={settingsModel.isNdsGovEnabled()}
                                    useCNRegionsOnly={useCNRegionsOnly}
                                    searchNodesEnabled={searchNodesEnabled}
                                />
                                <NDSClusterFormRegionSection
                                    providerOptions={providerOptions}
                                    updateReplicationSpecList={this.updateReplicationSpecList}
                                    provider={currentBackingProviders[0]}
                                    providerRegions={this.getProviderRegions(providerOptionsToUse, currentBackingProviders)}
                                    replicationSpecList={formValues.replicationSpecList}
                                    replicationSpec={selectedReplicationSpec}
                                    docsUrl={docsUrl}
                                    isTenant={isTenant}
                                    isEdit={isEdit}
                                    cloudContainers={cloudContainers}
                                    isPrivateIPModeEnabled={isPrivateIPModeEnabled}
                                    isMultiRegionConfigurationVisible={clusterBuilderFilterInterface.isMultiRegionConfigurationVisible}
                                    clusterType={formValues.clusterType}
                                    hasPITHighestPriorityRegionChanged={this.hasPITHighPriorityRegionChanged()}
                                    isNDSGovEnabled={settingsModel.isNdsGovEnabled()}
                                    useCNRegionsOnly={useCNRegionsOnly}
                                    backingProviderDefaultOneZoneReplicationSpec={backingProviderDefaultOneZoneReplicationSpec}
                                    deploymentType={deploymentType}
                                    setDeploymentType={setDeploymentType}
                                    ensureProvisionedIOPSVolumeType={this.ensureProvisionedIOPSVolumeType}
                                    diskSizeGB={formValues.diskSizeGB}
                                    isDiskSizeLargerThanStandardMax={isDiskSizeLargerThanStandardMax}
                                    clusterOutageSimulation={clusterOutageSimulation}
                                    turnOnAdvancedReplicationToggle={shouldTurnOnAdvancedToggle}
                                    hasDecoupledSearchFeatureEnabled={hasDecoupledSearchFeatureEnabled}
                                    searchNodesEnabled={searchNodesEnabled}
                                    searchConsiderationConfirmed={searchConsiderationConfirmed}
                                    searchDeploymentSpec={searchDeploymentSpec}
                                    setClusterFormValue={setClusterFormValue}
                                />
                            </Accordion>
                        )}
                    </ClusterTierLeftContainer>
                    {hasSelectedAdvancedTemplate && !isGeoZoned && (
                        <UseCaseBasedClusterTemplatesTooltipCard
                            adv_template={template as AdvancedTemplatesType}
                            data-testid="ucbctTooltip"
                        />
                    )}
                </ClusterTierContainer>
                {isGeoZoned && (
                    <div className="nds-cluster-form-divider">
                        <span className="nds-cluster-form-divider-text">Options Below Apply to All Zones</span>
                        <hr className="nds-cluster-form-divider-line" />
                    </div>
                )}
                <Accordion
                    ref={(ref) => {
                        this[ACCORDION_NAMES.INSTANCE_TIER.name] = ref;
                    }}
                    data-testid={ACCORDION_NAMES.INSTANCE_TIER.name}
                    headline={clusterTierHeader}
                    secondaryText={getClusterTierSubtext(providerOptionsToUse, this.props.formValues)}
                    secondarySubText={getClusterTierSecondarySubtext(
                        useInstanceSizeMaxStorage,
                        isNVMe,
                        this.props.formValues,
                        this.props.providerOptions,
                        this.props.crossCloudProviderOptions
                    )}
                    onHeadlineClick={this.toggleActive(ACCORDION_NAMES.INSTANCE_TIER)}
                    active={!!openAccordions[ACCORDION_NAMES.INSTANCE_TIER.name]}
                    highlightSecondaryText={highlightAccordions[ACCORDION_NAMES.INSTANCE_TIER.name]}
                    animate
                    inheritOverflow
                >
                    {!hasAnalyticsNodes && (
                        <NDSClusterFormInstanceSize
                            providerOptions={providerOptions}
                            crossCloudProviderOptions={crossCloudProviderOptions}
                            providers={currentBackingProviders}
                            clusterType={formValues.clusterType}
                            instanceSize={currentInstanceSizeName}
                            replicationSpecList={formValues.replicationSpecList}
                            setAutoScaling={(autoScaling) => setAutoScaling(autoScaling, DEFAULT_NODE_TYPE_SET)}
                            setClusterFormValue={setClusterFormValue}
                            setInstanceSize={(instanceSize) => setInstanceSize(instanceSize, DEFAULT_NODE_TYPE_SET)}
                            originalInstanceSize={originalInstanceSizeName}
                            isEdit={isEdit}
                            hasFreeCluster={hasFreeCluster}
                            isTenantUpgrade={isTenantUpgrade}
                            isCrossRegionEnabled={isCrossRegionEnabled}
                            isPrivateIPModeEnabled={isPrivateIPModeEnabled}
                            autoScaling={autoScaling}
                            diskSizeGB={formValues.diskSizeGB}
                            diskIOPS={replicationSpecListUtils.getIOPSToDisplay(
                                providerOptions,
                                formValues.diskSizeGB,
                                formValues.replicationSpecList
                            )}
                            originalProviders={originalBackingProviders}
                            useInstanceSizeMaxStorage={useInstanceSizeMaxStorage}
                            originalEncryptEBSVolume={
                                replicationSpecListUtils.getFirstProviderBaseHardwareSpec(
                                    originalCluster.replicationSpecList,
                                    CloudProvider.AWS
                                )?.encryptEBSVolume
                            }
                            encryptEBSVolume={
                                replicationSpecListUtils.getFirstProviderBaseHardwareSpec(
                                    formValues.replicationSpecList,
                                    CloudProvider.AWS
                                )?.encryptEBSVolume
                            }
                            ebsVolumeType={
                                replicationSpecListUtils.getFirstProviderBaseHardwareSpec(
                                    formValues.replicationSpecList,
                                    CloudProvider.AWS
                                )?.volumeType || null
                            }
                            diskBackupAllowed={diskBackupAllowed}
                            instanceClass={instanceClass}
                            cloudContainers={cloudContainers}
                            backupEnabled={formValues.backupEnabled}
                            isInstanceSizeVisible={isInstanceSizeVisible}
                            isNDSGovEnabled={settingsModel.isNdsGovEnabled()}
                            useCNRegionsOnly={useCNRegionsOnly}
                            selectedVersion={selectedVersion}
                            isAutoIndexingEnabled={settingsModel.isAutoIndexingEnabled()}
                            isAutoIndexingEligible={isAutoIndexingEligible}
                            isAwsGravitonEnabled={isAwsGravitonEnabled}
                            awsGravitonMinimumMongoDBVersion={settingsModel.getAwsGravitonMinimumMongoDBVersion()}
                            originalPreferredCpuArchitecture={originalClusterPreferredCpuArch}
                            isAnalyticsTier={false}
                            hasAnalyticsNodes={hasAnalyticsNodes}
                            shouldShowReplicationLagWarning={false}
                            areInstanceClassesAsymmetric={areInstanceClassesAsymmetric}
                            hasUnsetOplogMinRetentionHours={hasUnsetOplogMinRetentionHours}
                            isOriginalClusterSharded={isOriginalClusterSharded}
                        />
                    )}
                    {hasAnalyticsNodes && (
                        <Tabs aria-label="test" selected={selectedClusterTierTab} setSelected={this.setSelected}>
                            <Tab name="Base Tier">
                                <NDSClusterFormInstanceSize
                                    providerOptions={providerOptions}
                                    crossCloudProviderOptions={crossCloudProviderOptions}
                                    providers={currentBackingProviders}
                                    clusterType={formValues.clusterType}
                                    instanceSize={currentInstanceSizeName}
                                    replicationSpecList={formValues.replicationSpecList}
                                    setAutoScaling={(a) => setAutoScaling(a, new Set<NodeTypeFamily>().add(NodeTypeFamily.BASE))}
                                    setClusterFormValue={setClusterFormValue}
                                    setInstanceSize={(instanceSize, nodeTypeFamilySet) =>
                                        this.setInstanceSize(instanceSize, nodeTypeFamilySet)
                                    }
                                    originalInstanceSize={originalInstanceSizeName}
                                    isEdit={isEdit}
                                    hasFreeCluster={hasFreeCluster}
                                    isTenantUpgrade={isTenantUpgrade}
                                    isCrossRegionEnabled={isCrossRegionEnabled}
                                    isPrivateIPModeEnabled={isPrivateIPModeEnabled}
                                    autoScaling={autoScaling}
                                    diskSizeGB={formValues.diskSizeGB}
                                    diskIOPS={replicationSpecListUtils.getIOPSToDisplay(
                                        providerOptions,
                                        formValues.diskSizeGB,
                                        formValues.replicationSpecList
                                    )}
                                    originalProviders={originalBackingProviders}
                                    useInstanceSizeMaxStorage={useInstanceSizeMaxStorage}
                                    originalEncryptEBSVolume={
                                        replicationSpecListUtils.getFirstProviderBaseHardwareSpec(
                                            originalCluster.replicationSpecList,
                                            CloudProvider.AWS
                                        )?.encryptEBSVolume
                                    }
                                    encryptEBSVolume={
                                        replicationSpecListUtils.getFirstProviderBaseHardwareSpec(
                                            formValues.replicationSpecList,
                                            CloudProvider.AWS
                                        )?.encryptEBSVolume
                                    }
                                    ebsVolumeType={
                                        replicationSpecListUtils.getFirstProviderBaseHardwareSpec(
                                            formValues.replicationSpecList,
                                            CloudProvider.AWS
                                        )?.volumeType || null
                                    }
                                    diskBackupAllowed={diskBackupAllowed}
                                    instanceClass={instanceClass}
                                    cloudContainers={cloudContainers}
                                    backupEnabled={formValues.backupEnabled}
                                    isInstanceSizeVisible={isInstanceSizeVisible}
                                    isNDSGovEnabled={settingsModel.isNdsGovEnabled()}
                                    useCNRegionsOnly={useCNRegionsOnly}
                                    selectedVersion={selectedVersion}
                                    isAutoIndexingEnabled={settingsModel.isAutoIndexingEnabled()}
                                    isAutoIndexingEligible={isAutoIndexingEligible}
                                    isAwsGravitonEnabled={isAwsGravitonEnabled}
                                    awsGravitonMinimumMongoDBVersion={settingsModel.getAwsGravitonMinimumMongoDBVersion()}
                                    originalPreferredCpuArchitecture={originalClusterPreferredCpuArch}
                                    isAnalyticsTier={false}
                                    hasAnalyticsNodes={hasAnalyticsNodes}
                                    shouldShowReplicationLagWarning={false}
                                    areInstanceClassesAsymmetric={areInstanceClassesAsymmetric}
                                    hasUnsetOplogMinRetentionHours={hasUnsetOplogMinRetentionHours}
                                    isOriginalClusterSharded={isOriginalClusterSharded}
                                />
                            </Tab>
                            <Tab name={analyticsTierHeader}>
                                <NDSClusterFormInstanceSize
                                    providerOptions={providerOptions}
                                    crossCloudProviderOptions={crossCloudProviderOptions}
                                    providers={currentBackingProviders}
                                    clusterType={formValues.clusterType}
                                    instanceSize={currentAnalyticsInstanceSizeName}
                                    replicationSpecList={formValues.replicationSpecList}
                                    setAutoScaling={(a) => setAutoScaling(a, new Set<NodeTypeFamily>().add(NodeTypeFamily.ANALYTICS))}
                                    setClusterFormValue={setClusterFormValue}
                                    setInstanceSize={(instanceSize, nodeTypeFamilySet) =>
                                        this.setInstanceSize(instanceSize, nodeTypeFamilySet)
                                    }
                                    originalInstanceSize={originalAnalyticsInstanceSizeName}
                                    isEdit={isEdit}
                                    hasFreeCluster={hasFreeCluster}
                                    isTenantUpgrade={isTenantUpgrade}
                                    isCrossRegionEnabled={isCrossRegionEnabled}
                                    isPrivateIPModeEnabled={isPrivateIPModeEnabled}
                                    autoScaling={analyticsAutoScaling}
                                    diskSizeGB={formValues.diskSizeGB}
                                    diskIOPS={replicationSpecListUtils.getIOPSToDisplay(
                                        providerOptions,
                                        formValues.diskSizeGB,
                                        formValues.replicationSpecList
                                    )}
                                    originalProviders={originalBackingProviders}
                                    useInstanceSizeMaxStorage={useInstanceSizeMaxStorage}
                                    originalEncryptEBSVolume={
                                        replicationSpecListUtils.getFirstProviderAnalyticsHardwareSpec(
                                            originalCluster.replicationSpecList,
                                            CloudProvider.AWS
                                        )?.encryptEBSVolume
                                    }
                                    encryptEBSVolume={
                                        replicationSpecListUtils.getFirstProviderAnalyticsHardwareSpec(
                                            formValues.replicationSpecList,
                                            CloudProvider.AWS
                                        )?.encryptEBSVolume
                                    }
                                    ebsVolumeType={
                                        replicationSpecListUtils.getFirstProviderAnalyticsHardwareSpec(
                                            formValues.replicationSpecList,
                                            CloudProvider.AWS
                                        )?.volumeType || null
                                    }
                                    diskBackupAllowed={diskBackupAllowed}
                                    instanceClass={analyticsInstanceClass}
                                    cloudContainers={cloudContainers}
                                    backupEnabled={formValues.backupEnabled}
                                    isInstanceSizeVisible={isInstanceSizeVisible}
                                    isNDSGovEnabled={settingsModel.isNdsGovEnabled()}
                                    useCNRegionsOnly={useCNRegionsOnly}
                                    selectedVersion={selectedVersion}
                                    isAutoIndexingEnabled={settingsModel.isAutoIndexingEnabled()}
                                    isAutoIndexingEligible={isAutoIndexingEligible}
                                    isAwsGravitonEnabled={isAwsGravitonEnabled}
                                    awsGravitonMinimumMongoDBVersion={settingsModel.getAwsGravitonMinimumMongoDBVersion()}
                                    originalPreferredCpuArchitecture={originalClusterPreferredCpuArch}
                                    isAnalyticsTier
                                    hasAnalyticsNodes={hasAnalyticsNodes}
                                    shouldShowReplicationLagWarning={shouldShowReplicationLagWarning}
                                    areInstanceClassesAsymmetric={areInstanceClassesAsymmetric}
                                    hasUnsetOplogMinRetentionHours={hasUnsetOplogMinRetentionHours}
                                    isOriginalClusterSharded={isOriginalClusterSharded}
                                />
                            </Tab>
                        </Tabs>
                    )}
                </Accordion>
                <Accordion
                    ref={(ref) => {
                        this[ACCORDION_NAMES.ADDITIONAL_SETTINGS.name] = ref;
                    }}
                    headlineText={ACCORDION_NAMES.ADDITIONAL_SETTINGS.headlineText}
                    secondaryText={this.getAdditionalSettingsSecondaryText()}
                    secondarySubText={this.getBackupSubtext(diskBackupAllowed)}
                    onHeadlineClick={this.toggleActive(ACCORDION_NAMES.ADDITIONAL_SETTINGS)}
                    active={!!openAccordions[ACCORDION_NAMES.ADDITIONAL_SETTINGS.name]}
                    highlightSecondaryText={highlightAccordions[ACCORDION_NAMES.ADDITIONAL_SETTINGS.name]}
                    animate
                    hasSummary
                >
                    <>
                        <NDSClusterFormVersion
                            // NDSClusterFormVersion isn't conditionally rendered because it detects and handles
                            // case where selectedVersion wouldn't be in "Select a Version" dropdown, which can
                            // occur when NDSClusterFormVersion isn't visible.
                            selectedVersion={selectedVersion}
                            originalVersion={originalCluster.mongoDBMajorVersion}
                            providerVersions={possibleMongoDBMajorVersions}
                            defaultCDMongoDBVersion={providerOptionsToUse.defaultCDMongoDBVersion}
                            defaultCDMongoDBFCV={providerOptionsToUse.defaultCDMongoDBFCV}
                            fixedMongoDBFCV={originalCluster.fixedMongoDBFCV}
                            instanceSize={currentInstanceSizeName}
                            isEdit={isEdit}
                            isLDAPEnabled={isLDAPEnabled}
                            setClusterFormValue={setClusterFormValue}
                            customRoleVersions={customRoleVersions}
                            mongoDBMajorVersionsSubtext={crossCloudProviderOptions.mongoDBMajorVersionsSubtext || {}}
                            replicationSpecList={formValues.replicationSpecList}
                            isShardedCluster={formValues.clusterType !== ClusterType.REPLICASET}
                            versionToDeprecate={settingsModel.getVersionToDeprecate()}
                            versionDeprecatedByDate={settingsModel.getVersionDeprecatedByDate()}
                            isAllowDeprecatedClusterVersionFeatureFlagEnabled={settingsModel.hasAtlasAllowDeprecatedVersions()}
                            shouldShowOptimizedPrivateEndpointConnectionStringWarning={
                                updatedClusterWithPrivateEndpointSupportsOptimizedConnectionString &&
                                !!originalCluster.mongoDBMajorVersion &&
                                !isAtLeast5_0(originalCluster.mongoDBMajorVersion)
                            }
                            visible={isNotSharedOrVersionSelectionAvailable}
                            isCDFeatureFlagEnabled={settingsModel.hasProjectFeature('ATLAS_CONTINUOUS_DELIVERY')}
                            currentClusterVersionReleaseSystem={originalCluster.versionReleaseSystem}
                            selectedVersionReleaseSystem={formValues.versionReleaseSystem}
                        />
                        <hr className="nds-cluster-form-hr-thin" />
                    </>
                    <NDSClusterFormBackup
                        providers={currentProviders}
                        name={formValues.name}
                        groupId={settingsModel.getCurrentGroupId()}
                        docsUrl={docsUrl}
                        backupEnabled={formValues.backupEnabled}
                        diskBackupEnabled={formValues.diskBackupEnabled}
                        diskBackupAllowed={diskBackupAllowed}
                        tenantBackupEnabled={!!formValues.tenantBackupEnabled}
                        isEdit={isEdit}
                        isTenant={isTenant}
                        isFree={isFree}
                        hasEncryptionAtRestProvider={isClusterEncryptionAtRestEnabled}
                        setClusterFormValue={setClusterFormValue}
                        billingEstimate={billingEstimate}
                        isNVMe={isNVMe}
                        pitEnabled={formValues.pitEnabled}
                        hasPITBeenToggledOff={this.hasPITBeenToggledOff()}
                        hasPITBeenToggledOn={this.hasPITBeenToggledOn()}
                        mongoDBVersion={selectedVersion}
                        isMongoDBVersionUpgrade={isEdit && originalVersion !== selectedVersion}
                        onlyShowDiskBackup={onlyShowDiskBackup}
                        hasSnapshotDistribution={settingsModel.hasProjectFeature('CPS_SNAPSHOT_DISTRIBUTION_UI')}
                        originalCluster={originalCluster}
                        isNdsGovEnabled={settingsModel.isNdsGovEnabled()}
                        isBackupLockEnabled={backupUtils.isBackupLockEnabledForGaOrMVP(settingsModel, dataProtectionSettings)}
                        isBackupLockPitEnabled={backupUtils.isBackupLockPitEnabledForGaOrMVP(settingsModel, dataProtectionSettings)}
                    />
                    <hr className="nds-cluster-form-hr-thin" />
                    <NDSClusterFormTerminationProtection
                        terminationProtectionEnabled={formValues.terminationProtectionEnabled ?? false}
                        onTerminationProtectionChange={(enabled) => setClusterFormValue('terminationProtectionEnabled', enabled)}
                        isGroupOwner={settingsModel.isGroupOwner()}
                        isServerlessInstance={false}
                    />
                    {(clusterBuilderFilterInterface.advancedOptions.shardingVisible ||
                        clusterBuilderFilterInterface.advancedOptions.otherOptionsVisible) && (
                        <>
                            <hr className="nds-cluster-form-hr-thick" />
                            <div className="nds-cluster-form-setting nds-cluster-form-padding-top nds-cluster-form-subsection-header">
                                Advanced Settings
                            </div>
                            {!isGeoZoned && clusterBuilderFilterInterface.advancedOptions.shardingVisible && (
                                <>
                                    <NDSClusterFormSharding
                                        docsUrl={docsUrl}
                                        clusterType={formValues.clusterType}
                                        replicationSpec={selectedReplicationSpec}
                                        isEdit={isEdit}
                                        setClusterFormValue={setClusterFormValue}
                                        updateReplicationSpecList={this.updateReplicationSpecList}
                                        instanceSupportsSharding={shardingEnabled}
                                        minComputeInstanceSizeSupportsSharding={minComputeInstanceSizeSupportsSharding}
                                        isOriginalClusterSharded={isOriginalClusterSharded}
                                        originalInstanceSize={originalInstanceSizeName}
                                        shouldShowOptimizedPrivateEndpointConnectionStringWarning={
                                            updatedClusterWithPrivateEndpointSupportsOptimizedConnectionString &&
                                            originalCluster.clusterType === ClusterType.REPLICASET
                                        }
                                        minShardingInstanceSize={minShardingInstanceSize}
                                        isNdsGovEnabled={settingsModel.isNdsGovEnabled()}
                                    />
                                    <hr className="nds-cluster-form-hr-thin" />
                                </>
                            )}
                            {clusterBuilderFilterInterface.advancedOptions.otherOptionsVisible && (
                                <>
                                    <NDSClusterFormBIConnector
                                        docsUrl={docsUrl}
                                        biConnector={formValues.biConnector}
                                        instanceSize={currentInstanceSize.highCPUEquivalent || currentInstanceSize.name}
                                        biConnectorCostEstimate={biConnectorCostEstimate}
                                        isTenant={isTenant}
                                        setClusterFormValue={setClusterFormValue}
                                        hasAnalyticsNodes={hasAnalyticsNodes}
                                        originalClusterBiConnector={originalCluster.biConnector}
                                        isEdit={isEdit}
                                        isNdsGovEnabled={settingsModel.isNdsGovEnabled()}
                                    />
                                    {formValues.biConnector.enabled && (
                                        <>
                                            {showSqlInterfaceBanner && (
                                                <Banner
                                                    dismissible
                                                    css={{ marginRight: '40px', marginBottom: '20px' }}
                                                    onClose={this.onDismissSqlInterfaceBanner}
                                                >
                                                    <div>
                                                        <strong>Try the New Atlas SQL Interface Instead</strong>{' '}
                                                        <Badge variant={Variant.Green}>Preview</Badge>
                                                    </div>
                                                    The new{' '}
                                                    <DocsLink docsPath="query-with-sql" onClick={this.onClickSqlInterfaceDocs}>
                                                        Atlas SQL Interface
                                                    </DocsLink>{' '}
                                                    makes it easy to query your Atlas data using SQL. This interface is SQL-92 compatible and can
                                                    be accessed using the JDBC driver or the Tableau connector. Atlas SQL Interface is pay-per-use
                                                    since it leverages Atlas Data Federation for data processing.
                                                </Banner>
                                            )}
                                            <NDSClusterFormBIConnectorAdvanced
                                                setClusterFormValue={setClusterFormValue}
                                                processArgs={processArgs}
                                            />
                                        </>
                                    )}
                                    <hr className="nds-cluster-form-hr-thin" />
                                    <NDSClusterFormEncryptionAtRestProvider
                                        docsUrl={docsUrl}
                                        encryptionAtRestProvider={formValues.encryptionAtRestProvider}
                                        setClusterFormValue={setClusterFormValue}
                                        isEdit={isEdit}
                                        isTenant={isTenant}
                                        encryptionAtRest={encryptionAtRest}
                                        isHeadBackupEnabled={formValues.backupEnabled}
                                        defaultEncryptionAtRestProvider={
                                            defaultTemplates[currentBackingProviders[0]][defaultTemplateKey].encryptionAtRestProvider
                                        }
                                        originalEncryptionAtRestProvider={originalEncryptionAtRestProvider}
                                        useCNRegionsOnly={useCNRegionsOnly}
                                        isNdsGovEnabled={settingsModel.isNdsGovEnabled()}
                                        isBackupLockEAREnabled={backupUtils.isBackupLockEnabledForGAWithEAR(
                                            settingsModel,
                                            dataProtectionSettings
                                        )}
                                    />
                                    {!isTenant && (
                                        <NDSClusterFormAdvancedOptions
                                            processArgs={processArgs}
                                            originalProcessArgs={originalProcessArgs}
                                            diskSizeGB={formValues.diskSizeGB}
                                            setClusterFormValue={setClusterFormValue}
                                            mongoDBVersion={selectedVersion}
                                            clusterUsageStats={clusterUsageStats}
                                            autoScaling={autoScaling}
                                            isNVMe={isNVMe}
                                        />
                                    )}
                                </>
                            )}
                        </>
                    )}
                </Accordion>
                {!isEdit && (
                    <Accordion
                        ref={(ref) => {
                            this[ACCORDION_NAMES.NAME_CLUSTER.name] = ref;
                        }}
                        headlineText={ACCORDION_NAMES.NAME_CLUSTER.headlineText}
                        secondaryText={`${formValues.name || 'Please Enter a Name'}`}
                        onHeadlineClick={this.toggleActive(ACCORDION_NAMES.NAME_CLUSTER)}
                        active={!!openAccordions[ACCORDION_NAMES.NAME_CLUSTER.name]}
                        highlightSecondaryText={highlightAccordions[ACCORDION_NAMES.NAME_CLUSTER.name]}
                        errorSecondaryText={
                            !isNameValid ||
                            !isNameUnique ||
                            !isNamePrefixUnique ||
                            isNameEndWithHyphen ||
                            isNameInUseByClusterWithBackupSnapshots
                        }
                        animate
                    >
                        <NDSClusterFormName
                            setClusterFormValue={setClusterFormValue}
                            name={formValues.name}
                            isNameValid={isNameValid}
                            isNameUnique={isNameUnique}
                            isNamePrefixUnique={isNamePrefixUnique}
                            isNameEndWithHyphen={isNameEndWithHyphen}
                            isNameInUseByClusterWithBackupSnapshots={isNameInUseByClusterWithBackupSnapshots}
                            computeUnit="cluster"
                        />
                    </Accordion>
                )}
                {!isEdit && error && (
                    <div>
                        <ExceptionAlert error={error} />
                        <br />
                    </div>
                )}
            </div>
        );
    }
}

export default NDSClusterForm;
