import { useEffect } from "react";
import { useHistory } from "react-router-dom";

import apiService from "../../services/api.service";
import { useAuthState } from "../../common/useAuthContext";

// Where the server sends the browser once the provider callback has established
// a session. The session cookie is HttpOnly, so the only way to find out who it
// belongs to is to ask.
const OidcCallback = () => {
  const { dispatch } = useAuthState();
  const history = useHistory();

  useEffect(() => {
    let cancelled = false;
    dispatch({ type: "REQUEST_LOGIN" });

    apiService
      .me()
      .then((user) => {
        if (cancelled) return;
        dispatch({ type: "LOGIN_SUCCESS", payload: { user } });
        history.replace("/documents");
      })
      .catch((e) => {
        if (cancelled) return;
        dispatch({ type: "LOGIN_ERROR", error: "Login failed: " + e.message });
        history.replace("/login");
      });

    return () => {
      cancelled = true;
    };
  }, [dispatch, history]);

  return <div className="text-center mt-5">Finishing login...</div>;
};

export default OidcCallback;
