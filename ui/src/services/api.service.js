import constants from "../common/constants";

class ApiServices {
  header() {
    return {
      "Content-Type": "application/json",
    };
  }
  checkLogin() {
    if (localStorage.getItem("currentUser")) {
      return fetch(`${constants.ROOT_URL}/`, {
        method: "HEAD",
      }).then(handleError);
    }
  }
  login(loginData) {
    return fetch(`${constants.ROOT_URL}/login`, {
      method: "POST",
      headers: this.header(),
      body: JSON.stringify(loginData),
    })
      .then((r) => {
        if (r.status === 403) {
          throw new Error("This account has been disabled");
        }
        if (!r.ok) {
          throw new Error(r.statusText);
        }
      })
      .then(() => this.me());
  }
  // The session lives in an HttpOnly cookie, so the profile is asked for rather
  // than decoded here. This is also how the OIDC callback page finds out who it
  // has a session for.
  me() {
    return fetch(`${constants.ROOT_URL}/me`, {
      method: "GET",
      headers: this.header(),
    })
      .then((r) => {
        if (r.status === 401 || r.status === 403) {
          removeUser();
          throw new Error("Not authenticated");
        }
        if (!r.ok) {
          throw new Error(r.statusText);
        }
        return r.json();
      })
      .then((user) => {
        localStorage.setItem("currentUser", JSON.stringify(user));
        return user;
      });
  }
  // Served whether or not OIDC is configured; the bundle is compiled into the
  // binary and cannot read the server's environment.
  oidcInfo() {
    return fetch(`${constants.ROOT_URL}/oidc/info`, {
      method: "GET",
      headers: this.header(),
    }).then((r) => {
      if (!r.ok) {
        throw new Error(r.statusText);
      }
      return r.json();
    });
  }
  logout() {
    removeUser();
    return fetch(`${constants.ROOT_URL}/logout`);
  }

  upload(parent, files) {
    const formData = new FormData();
    formData.append("parent", parent);
    files.forEach((f) => {
      // file extensions which are not lowecase break the upload

      // set the file extension to be lower case
      let sub = f.name.split(".")
      const ext = sub[sub.length - 1].toLowerCase()
      sub.pop()
      sub.push(ext)

      // copy file data to new (lowecase) file
      const newFile = new File([f], sub.join("."), {type: f.type});
      formData.append("file", newFile);
    });

    return fetch(`${constants.ROOT_URL}/documents/upload`, {
      method: "POST",
      body: formData,
    }).then(async (r) => {
      if (r.status === 409) {
        const body = await r.json();
        const err = new Error(body.error);
        err.status = 409;
        err.docId = body.docId;
        throw err;
      }
      if (!r.ok) {
        const body = await r.json();
        throw new Error(body.error || r.statusText);
      }
      return r.json();
    });
  }

  listPasscodeResets() {
    return fetch(`${constants.ROOT_URL}/passcode/resets`, {
      method: "GET",
      headers: this.header(),
    }).then((r) => {
      handleError(r);
      return r.json();
    });
  }
  approvePasscodeReset(uuid) {
    return fetch(`${constants.ROOT_URL}/passcode/resets/${uuid}/approve`, {
      method: "POST",
      headers: this.header(),
    }).then((r) => handleError(r));
  }
  dismissPasscodeReset(uuid) {
    return fetch(`${constants.ROOT_URL}/passcode/resets/${uuid}`, {
      method: "DELETE",
      headers: this.header(),
    }).then((r) => handleError(r));
  }

  resetPassword(resetPasswordForm) {
    return fetch(`${constants.ROOT_URL}/profile`, {
      method: "POST",
      headers: this.header(),
      body: JSON.stringify({
        ...resetPasswordForm,
      }),
    });
  }

  listDocument() {
    return fetch(`${constants.ROOT_URL}/documents`, {
      method: "GET",
      headers: this.header(),
    }).then((r) => {
      handleError(r);
      return r.json();
    });
  }
  getCode() {
    return fetch(`${constants.ROOT_URL}/newcode`, {
      method: "GET",
      headers: this.header(),
    }).then((r) => {
      handleError(r);
      return r.json();
    });
  }

  deleteDocument(id) {
    return fetch(`${constants.ROOT_URL}/documents/${id}`, {
      method: "DELETE",
      headers: this.header(),
    }).then((r) => handleError(r));
  }
  download(id, exportType) {
    let url = `${constants.ROOT_URL}/documents/${id}`;
    if (exportType) url += `?type=${exportType}`;
    return fetch(url, {
      method: "GET",
    }).then((r) => {
      handleError(r);
      return r.blob();
    });
  }

  createFolder(data) {
    return fetch(`${constants.ROOT_URL}/folders`, {
      method: "POST",
      headers: this.header(),
      body: JSON.stringify(data),
    }).then((r) => {
      handleError(r);
      return r.json();
    });
  }
  updateuser(usr) {
    return fetch(`${constants.ROOT_URL}/users`, {
      method: "PUT",
      headers: this.header(),
      body: JSON.stringify(usr),
    }).then((r) => handleError(r));
  }
  createuser(usr) {
    return fetch(`${constants.ROOT_URL}/users`, {
      method: "POST",
      headers: this.header(),
      body: JSON.stringify(usr),
    }).then((r) => handleError(r));
  }
  setuserdisabled(userid, disabled) {
    return fetch(`${constants.ROOT_URL}/users`, {
      method: "PUT",
      headers: this.header(),
      body: JSON.stringify({ userid, disabled }),
    }).then((r) => handleError(r));
  }
  deleteuser(userid) {
    return fetch(`${constants.ROOT_URL}/users/${userid}`, {
      method: "DELETE",
      headers: this.header(),
    }).then((r) => handleError(r));
  }

  listtemplates() {
    return fetch(`${constants.ROOT_URL}/templates`, {
      method: "GET",
      headers: this.header(),
    }).then((r) => {
      handleError(r);
      return r.json();
    });
  }

  uploadtemplates(files) {
    const formData = new FormData();
    files.forEach((f) => formData.append("file", f));

    return fetch(`${constants.ROOT_URL}/templates`, {
      method: "POST",
      body: formData,
    }).then(async (r) => {
      if (!r.ok) {
        const body = await r.json().catch(() => ({}));
        throw new Error(body.error || r.statusText);
      }
      return r.json();
    });
  }

  deletetemplate(name) {
    return fetch(`${constants.ROOT_URL}/templates/${encodeURIComponent(name)}`, {
      method: "DELETE",
      headers: this.header(),
    }).then((r) => handleError(r));
  }

  listintegration() {
    return fetch(`${constants.ROOT_URL}/integrations`, {
      method: "GET",
      headers: this.header(),
    }).then((r) => {
      handleError(r);
      return r.json();
    });
  }
  updateintegration(integration) {
    return fetch(`${constants.ROOT_URL}/integrations/${integration.id}`, {
      method: "PUT",
      headers: this.header(),
      body: JSON.stringify(integration),
    }).then((r) => handleError(r));
  }
  createintegration(integration) {
    return fetch(`${constants.ROOT_URL}/integrations`, {
      method: "POST",
      headers: this.header(),
      body: JSON.stringify(integration),
    }).then((r) => handleError(r));
  }
  deleteintegration(integrationid) {
    return fetch(`${constants.ROOT_URL}/integrations/${integrationid}`, {
      method: "DELETE",
      headers: this.header(),
    }).then((r) => handleError(r));
  }
}

function removeUser(){
  localStorage.removeItem("currentUser");
}
function handleError(r) {
  if (!r.ok) {
    if (r.status === 401) {
      removeUser();
      window.location.reload(true);
      return
    }
    if (r.headers.get("Content-Type").startsWith("application/json")) {
      return r.json().then(d => {throw new Error(d.error)});
    }
    if (r.status === 400) {
      return r.text().then(text => {throw new Error(text)})
    }
    return Promise.reject(r.status)
  }
}

const apiServices = new ApiServices()
export default apiServices
