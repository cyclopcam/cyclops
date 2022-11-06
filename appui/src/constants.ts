// dummyMode is intended to allow one to develop the app on localhost
// in Chrome, without having to actually run everything inside an Android app.
export const dummyMode = window.location.hostname == "localhost";
console.log(`dummyMode: ${dummyMode}, hostname: ${window.location.hostname}`);

export const panelSlideTransitionMS = 200;