import { Link } from "react-router-dom";
import { useAuth } from "../context/AuthContext";
import Container from "react-bootstrap/Container";
import Navbar from "react-bootstrap/Navbar";
import Nav from "react-bootstrap/Nav";
import Image from "react-bootstrap/Image";
import { FaIgloo, FaFilm } from "react-icons/fa";
import logo from "../assets/images/logo-alt.png";

export default function Header() {
  const { user } = useAuth();

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
              </Navbar.Collapse>
            </>
          )}
        </Container>
      </Navbar>
    </header>
  );
}
