// A block definition diagram whose composition edges follow Cameo's tree
// routing: every edge leaves the whole down one trunk, runs along one bus and
// drops to its part, so the routes share their first segments and only the
// last segment before each part is the edge's own. Two parts share a type, so
// two routes end at one box.
package Microscope {
    part def ImagingParameters;
    part def FocusSettings;
    part def ElectronDetector;
    part def ObjectiveLens;
    part def Stigmator;

    part def ScanningElectronMicroscope {
        part imagingParameters : ImagingParameters;
        part focusSettings : FocusSettings;
        part backscatteredElectronDetector : ElectronDetector;
        part secondaryElectronDetector : ElectronDetector;
        part objectiveLens : ObjectiveLens;
        part stigmator : Stigmator;
    }
}

package MicroscopeViews {
    private import Views::*;
    private import DiagramLayout::*;

    view structure {
        expose Microscope::ScanningElectronMicroscope;
        expose Microscope::ImagingParameters;
        expose Microscope::FocusSettings;
        expose Microscope::ElectronDetector;
        expose Microscope::ObjectiveLens;
        expose Microscope::Stigmator;
        @Canvas { unit = "px"; width = 1100; height = 420; }
        metadata Layout about Microscope::ScanningElectronMicroscope { x = 400; y = 20; width = 300; height = 80; }
        metadata Layout about Microscope::ImagingParameters { x = 20; y = 300; width = 180; height = 70; }
        metadata Layout about Microscope::FocusSettings { x = 230; y = 300; width = 160; height = 70; }
        metadata Layout about Microscope::ElectronDetector { x = 420; y = 300; width = 200; height = 70; }
        metadata Layout about Microscope::ObjectiveLens { x = 660; y = 300; width = 160; height = 70; }
        metadata Layout about Microscope::Stigmator { x = 860; y = 300; width = 160; height = 70; }
        metadata Route about Microscope::ScanningElectronMicroscope::imagingParameters { points = (550, 100, 550, 200, 110, 200, 110, 300); }
        metadata Route about Microscope::ScanningElectronMicroscope::focusSettings { points = (550, 100, 550, 200, 310, 200, 310, 300); }
        metadata Route about Microscope::ScanningElectronMicroscope::backscatteredElectronDetector { points = (550, 100, 550, 200, 480, 200, 480, 300); }
        metadata Route about Microscope::ScanningElectronMicroscope::secondaryElectronDetector { points = (550, 100, 550, 200, 560, 200, 560, 300); }
        metadata Route about Microscope::ScanningElectronMicroscope::objectiveLens { points = (550, 100, 550, 200, 740, 200, 740, 300); }
        metadata Route about Microscope::ScanningElectronMicroscope::stigmator { points = (550, 100, 550, 200, 940, 200, 940, 300); }
    }
}
