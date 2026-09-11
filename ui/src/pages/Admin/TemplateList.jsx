import { useRef, useState } from "react";
import { toast } from "react-toastify";
import { Alert, Button, Card, Table } from "react-bootstrap";

import useFetch from "../../hooks/useFetch";
import Spinner from "../../components/Spinner";
import apiService from "../../services/api.service";

const templateListUrl = "templates";

export default function TemplateList() {
  const [index, setIndex] = useState(0);
  const { data, error, loading } = useFetch(templateListUrl, index);
  const [busy, setBusy] = useState(false);
  const fileInput = useRef(null);

  const refresh = () => setIndex((previous) => previous + 1);

  function upload(event) {
    const files = Array.from(event.target.files || []);
    if (files.length === 0) {
      return;
    }

    setBusy(true);
    apiService
      .uploadtemplates(files)
      .then((result) => {
        toast.success(`Added ${result.stored.join(", ")}`);
        refresh();
      })
      .catch((e) => toast.error(e.message))
      .finally(() => {
        setBusy(false);
        if (fileInput.current) {
          fileInput.current.value = "";
        }
      });
  }

  function remove(name) {
    setBusy(true);
    apiService
      .deletetemplate(name)
      .then(() => {
        toast.success(`Removed ${name}`);
        refresh();
      })
      .catch((e) => toast.error(e.message))
      .finally(() => setBusy(false));
  }

  if (loading) {
    return <Spinner />;
  }

  if (error) {
    return <Alert variant="danger">{error.message || String(error)}</Alert>;
  }

  const templates = (data && data.templates) || [];

  return (
    <Card className="mt-3">
      <Card.Header>Page templates</Card.Header>
      <Card.Body>
        <p className="text-muted">
          Templates are what the tablet draws under your handwriting. They never
          travel over sync, so a copy has to live here for exports to show them.
          A template you add here must be installed on the tablet as well, or
          the tablet will not offer it when you make a page.
        </p>
        {data && data.directory ? (
          <p className="text-muted">
            Kept in <code>{data.directory}</code>
          </p>
        ) : null}

        <input
          ref={fileInput}
          type="file"
          accept=".template"
          multiple
          onChange={upload}
          disabled={busy}
          className="mb-3 form-control"
        />

        {templates.length === 0 ? (
          <Alert variant="secondary">
            No templates yet. Pages are exported without one, the way they
            always have been. Copy them from a tablet, they are in
            <code> /usr/share/remarkable/templates</code>.
          </Alert>
        ) : (
          <Table striped hover size="sm">
            <thead>
              <tr>
                <th>Name</th>
                <th className="text-end">Remove</th>
              </tr>
            </thead>
            <tbody>
              {templates.map((name) => (
                <tr key={name}>
                  <td>{name}</td>
                  <td className="text-end">
                    <Button
                      variant="outline-danger"
                      size="sm"
                      disabled={busy}
                      onClick={() => remove(name)}
                    >
                      Remove
                    </Button>
                  </td>
                </tr>
              ))}
            </tbody>
          </Table>
        )}
      </Card.Body>
    </Card>
  );
}
