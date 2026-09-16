import { Button } from "react-bootstrap";
import { useHistory } from "react-router-dom";

// A landing page that exists so logging out actually sticks. With local login
// disabled the server sends any unauthenticated page straight to the provider,
// and the provider still has its own session, so landing anywhere else would
// sign the user back in before they saw that they had left.
const LoggedOut = () => {
  const history = useHistory();

  return (
    <div className="text-center mt-5">
      <p>You have been logged out.</p>
      <Button onClick={() => history.push("/login")}>Log in again</Button>
    </div>
  );
};

export default LoggedOut;
