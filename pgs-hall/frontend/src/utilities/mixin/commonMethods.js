export function getDeviceWidth() {
    try {
        return (window.innerWidth > 0) ? window.innerWidth : screen.width;
    } catch (e) {
        return 0;
    }
}

export function getDeviceHeight() {
    try {
        return (window.innerHeight > 0) ? window.innerHeight : screen.height;
    } catch (e) {
        return 0;
    }
}

export function instanceHeaders(token) {

    const lang = localStorage.getItem("LOCALE");
    const headers = {
        "content-type": "application/json",
        "X-target-device-width" : getDeviceWidth(),
        "X-target-device-height" : getDeviceHeight(),
        "access_token": token,
        "locale": lang && lang != null && lang !== '' ? lang : ""
    };

    return headers;
}

export function setDocumentTitle(title) {
    try
    {
        window.setDocumentTitle(title);
        window.parent.setDocumentTitle(title);
    }
    catch(ex)
    {
        console.log(ex);
    }
}


export function getGlobalSettings() {

    const settings = {
        "encrypt_salt_key": "z85pc7w42a6dc",
    };

    return settings;
}