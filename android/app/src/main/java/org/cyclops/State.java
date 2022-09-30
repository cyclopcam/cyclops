package org.cyclops;

import android.util.Log;

import java.util.ArrayList;

class State {
    static final State global = new State();

    class Server {
        String publicKey = "";
    }

    Scanner scanner;
    ArrayList<Server> servers = new ArrayList<Server>();

    State() {
        Log.i("C", "Global state init");
    }
}
