package org.cyclops;

import android.util.Log;

import java.util.ArrayList;

class State {
    static final State global = new State();

    class Server {
        String publicKey = "";
    }

    // These objects are created in MainActivity's onCreate
    Scanner scanner;
    Connector connector;

    ArrayList<Server> servers = new ArrayList<Server>();

    State() {
        Log.i("C", "Global state init");
    }
}
