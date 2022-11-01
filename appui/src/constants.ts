// debugMode is intended to allow one to develop the app on localhost
// in Chrome, without having to actually run everything inside an Android app.
export const debugMode = window.location.hostname == "localhost";
console.log(debugMode, window.location.hostname);

export const panelSlideTransitionMS = 200;