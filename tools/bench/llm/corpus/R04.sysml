package Camera {
    private import ScalarValues::*;

    part def Camera {
        attribute resolutionMegapixels : Real;
        attribute hasFlash : Boolean;
    }

    requirement def ResolutionRequirement {
        doc /* The camera shall capture images of at least 12 megapixels. */
        subject camera : Camera;
        require constraint { camera.resolutionMegapixels >= 12.0 }
    }

    requirement def FlashRequirement {
        doc /* The camera shall have a built-in flash. */
        subject camera : Camera;
        require constraint { camera.hasFlash }
    }

    part phoneCamera : Camera {
        attribute redefines resolutionMegapixels = 48.0;
        attribute redefines hasFlash = true;
    }

    satisfy requirement resolution : ResolutionRequirement by phoneCamera;
    satisfy requirement flash : FlashRequirement by phoneCamera;
}
