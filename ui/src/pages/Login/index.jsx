import React, { useEffect, useState } from "react";
import { useHistory } from "react-router-dom";
import { Button, Form } from "react-bootstrap";

import { useAuthState } from "../../common/useAuthContext";
import { loginUser } from "../../common/actions";
import apiService from "../../services/api.service";

import styles from "./Login.module.scss";

const Login = () => {
  let history = useHistory();
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  // Until the server answers, assume the password form: it is what every
  // instance without OIDC has, and it avoids a blank page on a failed probe.
  const [oidc, setOidc] = useState({
    enabled: false,
    displayName: "Login with OIDC",
    localLoginEnabled: true,
  });

  const { state, dispatch } = useAuthState(); //read the values of loading and errorMessage from context
  const { errorMessage, loading } = state;

  useEffect(() => {
    let cancelled = false;
    apiService
      .oidcInfo()
      .then((info) => {
        if (!cancelled) setOidc(info);
      })
      .catch(() => {
        // Leave the default: a reachable server with no OIDC looks the same.
      });
    return () => {
      cancelled = true;
    };
  }, []);

  const handleLogin = async (e) => {
    e.preventDefault();

    let payload = { email: username, password };
    try {
      await loginUser(dispatch, payload);
      history.push("/documents"); //TODO: usenavigate or return redirect
    } catch (error) {
      console.log(error);
    }
  };

  return (
    <div className={styles.container}>
      <div className={styles.formContainer}>
        {errorMessage ? <p className={styles.error}>{errorMessage}</p> : null}

        {oidc.localLoginEnabled && (
          <Form>
            <Form.Group className="mb-3">
              <Form.Label htmlFor="username">Username</Form.Label>
              <Form.Control
                id="username"
                value={username}
                autoFocus
                onChange={(e) => setUsername(e.target.value)}
                disabled={loading}
                placeholder="Username"
                autoComplete="username"
              />
            </Form.Group>

            <Form.Group className="mb-3">
              <Form.Label htmlFor="password">Password</Form.Label>
              <Form.Control
                type="password"
                id="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                disabled={loading}
                placeholder="Password"
                autoComplete="current-password"
              />
            </Form.Group>

            <Button type="submit" onClick={handleLogin} disabled={loading}>
              Login
            </Button>
          </Form>
        )}

        {oidc.enabled && oidc.localLoginEnabled && <hr />}

        {oidc.enabled && (
          // A plain link, not a fetch: this is a top level navigation to the
          // provider's own origin and has to leave the single page app.
          <Button
            variant={oidc.localLoginEnabled ? "outline-primary" : "primary"}
            href="/ui/api/oidc/login"
            disabled={loading}
          >
            {oidc.displayName}
          </Button>
        )}
      </div>
    </div>
  );
};

export default Login;
