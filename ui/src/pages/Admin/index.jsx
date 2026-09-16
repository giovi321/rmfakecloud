import Container from "react-bootstrap/Container";
import Stack from "react-bootstrap/Stack";
import UserList from "./UserList";
import TemplateList from "./TemplateList";

const Home = () => {
  return (
    <Container fluid style={{ height: "100%", overflowY: "auto" }}>
      <Stack className="pb-4">
          <UserList />
          <TemplateList />
      </Stack>
    </Container>
  );
};

export default Home;
