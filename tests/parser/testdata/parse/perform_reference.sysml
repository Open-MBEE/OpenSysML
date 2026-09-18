package test {

	action def TakePicture;

	action takePicture : TakePicture {
		action focus;
		action shoot;
	}

	part camera {

		perform action takePhoto references takePicture;

		part focuser {
			perform takePhoto.focus;
		}
	}

	part torqueGenerator;

	allocate takePicture to camera {
		allocate takePicture.focus to camera.focuser;
	}
}
