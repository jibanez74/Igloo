import { Link, useNavigate } from "react-router-dom";
import { useAuth } from "../context/AuthContext";
import Container from "react-bootstrap/Container";
import Navbar from "react-bootstrap/Navbar";
import Nav from "react-bootstrap/Nav";
import NavDropdown from "react-bootstrap/NavDropdown";
import Image from "react-bootstrap/Image";
import api from "../lib/api";
import {
  FaCog,
  FaIgloo,
  FaFilm,
  FaUserCircle,
  FaSignOutAlt,
} from "react-icons/fa";
import logo from "../assets/images/logo-alt.png";

export default function Header() {
  const { user, setUser } = useAuth();

  const navigate = useNavigate();

  const handleSignOut = async () => {
    try {
      await api.get("/logout");

      setUser(null);
      navigate("/login", { replace: true });
    } catch (err) {
      console.error(err);
    }
  };

  return (
    <header>
      <Navbar fixed='top' expand='md' data-bs-theme='dark'>
        <Container>
          <Navbar.Brand as={Link} to='/'>
            <Image
              src={logo}
              height='30'
              className='d-inline-block align-top'
              alt='Igloo Logo'
            />
          </Navbar.Brand>

          {user && (
            <>
              <Navbar.Toggle aria-controls='nav-res' />

              <Navbar.Collapse id='nav-res'>
                <Nav className='me-auto'>
                  <Nav.Link as={Link} to='/'>
                    <FaIgloo className='me-2' />
                    Home
                  </Nav.Link>

                  <Nav.Link as={Link} to='/movies'>
                    <FaFilm className='me-2' />
                    Movies
                  </Nav.Link>
                </Nav>

                <Nav className='ms-auto'>
                  <NavDropdown
                    title={
                      <span>
                        <FaUserCircle className='me-2' />
                        {user.name}
                      </span>
                    }
                    id='nav-dropdown'
                  >
                    <NavDropdown.Item as={Link} to='/settings'>
                      <FaCog className='me-2' />
                      Settings
                    </NavDropdown.Item>

                    <NavDropdown.Divider />

                    <NavDropdown.Item onClick={handleSignOut}>
                      <FaSignOutAlt className='me-2' />
                      Sign Out
                    </NavDropdown.Item>
                  </NavDropdown>
                </Nav>
              </Navbar.Collapse>
            </>
          )}
        </Container>
      </Navbar>
    </header>
  );
}
