// Feature::owningFeatureMembership, endOwningType, ownedTypeFeaturing, ownedFeatureInverting,
// ownedFeatureChaining, featureTarget, crossFeature and the variant properties on meta values.
package test {
    private import ScalarValues::*;
    private import SequenceFunctions::*;

    part def A;
    part def B;
    connection def Link {
        end part a : A;
        end part b : B;
    }
    part def Sys {
        part x : A;
        part y : B;
        variation part v : A {
            variant part v1 : A;
            variant part v2 : A;
            assert constraint { (v meta SysML::PartUsage).variant->size() == 2 }
        }
        connection c : Link connect a ::> x to b ::> y;
    }
    enum def Color { red; green; }
    part top : A;

    part def Report {
        attribute xOwningFeatureMemberships = (Sys::x meta SysML::PartUsage).owningFeatureMembership->size();
        attribute topOwningFeatureMemberships = (top meta SysML::PartUsage).owningFeatureMembership->size();
        attribute xEndOwningTypes = (Sys::x meta SysML::PartUsage).endOwningType->size();
        attribute aEndOwningTypes = (Link::a meta SysML::PartUsage).endOwningType->size();
        attribute cEndOwningTypes = (Sys::c meta SysML::ConnectionUsage).connectorEnd.endOwningType->size();
        attribute xTypeFeaturings = (Sys::x meta SysML::PartUsage).ownedTypeFeaturing->size();
        attribute xFeatureInvertings = (Sys::x meta SysML::PartUsage).ownedFeatureInverting->size();
        attribute xFeatureChainings = (Sys::x meta SysML::PartUsage).ownedFeatureChaining->size();
        attribute xFeatureTargets = (Sys::x meta SysML::PartUsage).featureTarget->size();
        attribute xCrossFeatures = (Sys::x meta SysML::PartUsage).crossFeature->size();
        attribute vVariantMemberships = (Sys::v meta SysML::PartUsage).variantMembership->size();
        attribute vVariants = (Sys::v meta SysML::PartUsage).variant->size();
        attribute xVariants = (Sys::x meta SysML::PartUsage).variant->size();
        attribute colorVariants = (Color meta SysML::EnumerationDefinition).variant->size();
        attribute aVariants = (A meta SysML::PartDefinition).variant->size();
    }
}
