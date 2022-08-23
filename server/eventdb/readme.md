# EventDB

EventDB is a database of video recordings.

# EventDB Notes

* We incorporate `camera_id` into the `recordings` table. This points to the camera from the config DB.
  If you wanted to copy or move recordings from one EventDB to another, you'd need to take special care of the camera IDs.

  