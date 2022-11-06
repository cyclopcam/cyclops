package org.cyclops;

import android.content.Context;
import android.database.sqlite.SQLiteDatabase;
import android.database.sqlite.SQLiteOpenHelper;
import android.util.Log;

public class LocalDB extends SQLiteOpenHelper {
    // If you change the database schema, you must increment the database version.
    public static final int DATABASE_VERSION = 2;
    public static final String DATABASE_NAME = "cyclops.db";

    public LocalDB(Context context) {
        super(context, DATABASE_NAME, null, DATABASE_VERSION);
    }

    public void onCreate(SQLiteDatabase db) {
        Log.i("C", "Creating cyclops.db");
        onUpgrade(db, 0, SQL_MIGRATIONS.length);
    }

    public void onUpgrade(SQLiteDatabase db, int oldVersion, int newVersion) {
        for (int i = oldVersion; i < newVersion; i++) {
            Log.i("C", "Running cyclops.db migration " + i);
            db.execSQL(SQL_MIGRATIONS[i]);
        }
    }

    public void onDowngrade(SQLiteDatabase db, int oldVersion, int newVersion) {
        //onUpgrade(db, oldVersion, newVersion);
    }

    private static final String SQL_MIGRATE_1 = "CREATE TABLE server (publicKey TEXT PRIMARY KEY, lanIP TEXT, bearerToken TEXT);";
    private static final String SQL_MIGRATE_2 = "ALTER TABLE server ADD COLUMN name TEXT;";
    public static final String[] SQL_MIGRATIONS = {
            SQL_MIGRATE_1,
            SQL_MIGRATE_2,
    };
}

