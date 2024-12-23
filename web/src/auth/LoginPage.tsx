import { useState } from "react";
import { Navigate } from "react-router-dom";
import { useAuth } from "../context/AuthContext";
import Container from "react-bootstrap/Container";
import Row from "react-bootstrap/Row";
import Col from "react-bootstrap/Col";
import Card from "react-bootstrap/Card";
import Form from "react-bootstrap/Form";
import InputGroup from "react-bootstrap/InputGroup";
import Button from "react-bootstrap/Button";
import { FaUser, FaEnvelope, FaLock } from "react-icons/fa";
import Message from "../shared/Message";
import getError from "../lib/getError";
import api from "../lib/api";
import type { User } from "../types/User";

type AuthResponse = {
  user: User;
};

export default function LoginPage() {
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  const { user, setUser } = useAuth();

  const handleSubmit = async (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    setLoading(true);
    if (error) setError("");

    const formData = new FormData(e.currentTarget);

    try {
      const { data } = await api.post<AuthResponse>("/auth/login", {
        username: formData.get("username"),
        email: formData.get("email"),
        password: formData.get("password"),
      });

      if (!data.user) {
        throw new Error("no user was returned from the server");
      }

      setUser(data.user);
    } catch (err) {
      setError(getError(err));
    } finally {
      setLoading(false);
    }
  };

  if (user) {
    return <Navigate to='/' replace />;
  }

  return (
    <Container className='py-5'>
      <Row className='justify-content-center'>
        <Col xs={11} sm={8} md={6} lg={4}>
          <Card bg='primary' text='light' className='shadow border-secondary'>
            <Card.Body className='p-4'>
              <h2 className='text-center mb-4'>Login</h2>

              {error && <Message msg={error} />}

              <Form onSubmit={handleSubmit}>
                <Form.Group className='mb-3' controlId='username'>
                  <Form.Label>Username</Form.Label>
                  <InputGroup>
                    <InputGroup.Text className='bg-dark border-secondary text-light'>
                      <FaUser />
                    </InputGroup.Text>
                    <Form.Control
                      autoFocus
                      autoCapitalize='off'
                      name='username'
                      type='text'
                      placeholder='Enter username'
                      required
                      minLength={2}
                      maxLength={20}
                      className='bg-dark border-secondary text-light'
                      disabled={loading}
                    />
                  </InputGroup>
                </Form.Group>

                <Form.Group className='mb-3' controlId='email'>
                  <Form.Label>Email</Form.Label>
                  <InputGroup>
                    <InputGroup.Text className='bg-dark border-secondary text-light'>
                      <FaEnvelope />
                    </InputGroup.Text>
                    <Form.Control
                      name='email'
                      type='email'
                      placeholder='Enter email'
                      required
                      className='bg-dark border-secondary text-light'
                      disabled={loading}
                    />
                  </InputGroup>
                </Form.Group>

                <Form.Group className='mb-4' controlId='password'>
                  <Form.Label>Password</Form.Label>
                  <InputGroup>
                    <InputGroup.Text className='bg-dark border-secondary text-light'>
                      <FaLock />
                    </InputGroup.Text>
                    <Form.Control
                      name='password'
                      type='password'
                      placeholder='Enter password'
                      required
                      minLength={9}
                      maxLength={128}
                      className='bg-dark border-secondary text-light'
                      disabled={loading}
                    />
                  </InputGroup>
                </Form.Group>

                <Button
                  variant='primary'
                  type='submit'
                  className='w-100'
                  disabled={loading}
                >
                  {loading ? "Logging in..." : "Login"}
                </Button>
              </Form>
            </Card.Body>
          </Card>
        </Col>
      </Row>
    </Container>
  );
}
