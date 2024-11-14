import { useRef, useState } from "react";
import {
  View,
  StyleSheet,
  Image,
  Pressable,
  TextInput,
  useTVEventHandler,
} from "react-native";
import { Ionicons } from "@expo/vector-icons";
import Colors from "@/constants/Colors";
import ThemedView from "@/components/ThemedView";
import ThemedText from "@/components/ThemedText";
import useScale from "@/hooks/useScale";
import api from "@/lib/api";

const BASE_INPUT_HEIGHT = 60;
const BASE_PADDING = 40;
const BASE_SPACING = 20;

export default function LoginScreen() {
  const scale = useScale();

  const inputHeight = BASE_INPUT_HEIGHT * scale;
  const padding = BASE_PADDING * scale;
  const spacing = BASE_SPACING * scale;

  const usernameInputRef = useRef<TextInput>(null);
  const emailInputRef = useRef<TextInput>(null);
  const passwordInputRef = useRef<TextInput>(null);
  const signInButtonRef = useRef<View>(null);

  const [username, setUsername] = useState('');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [focusedElement, setFocusedElement] = useState<string | null>(null);
  const [showPassword, setShowPassword] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useTVEventHandler(event => {
    if (event.eventType === "right") {
      if (focusedElement === "username") {
        emailInputRef.current?.focus();
        setFocusedElement("email");
      } else if (focusedElement === "email") {
        passwordInputRef.current?.focus();
        setFocusedElement("password");
      } else if (focusedElement === "password") {
        signInButtonRef.current?.focus();
        setFocusedElement("signIn");
      }
    } else if (event.eventType === "left") {
      if (focusedElement === "signIn") {
        passwordInputRef.current?.focus();
        setFocusedElement("password");
      } else if (focusedElement === "password") {
        emailInputRef.current?.focus();
        setFocusedElement("email");
      } else if (focusedElement === "email") {
        usernameInputRef.current?.focus();
        setFocusedElement("username");
      }
    }
  });

  const handleLogin = async () => {
    try {
      setLoading(true);
      setError(null);

      const response = await api.post("/login", {
        username,
        email,
        password,
      });

      console.log(response.data);
      // Handle successful login
    } catch (err) {
      setError("Invalid credentials. Please try again.");
    } finally {
      setLoading(false);
    }
  };

  return (
    <ThemedView variant='dark' style={styles.container}>
      <View style={[styles.formContainer, { padding }]}>
        {/* Logo */}
        <Image
          source={require("../assets/images/logo-alt.png")}
          style={[styles.logo, { marginBottom: spacing * 2 }]}
          resizeMode='contain'
        />

        {/* Title */}
        <ThemedText
          variant='light'
          size='xlarge'
          weight='bold'
          style={{ marginBottom: spacing * 2 }}
        >
          Sign In
        </ThemedText>

        {/* Form Fields */}
        <View style={[styles.inputContainer, { marginBottom: spacing }]}>
          <Ionicons
            name='person-outline'
            size={24}
            color={Colors.info}
            style={styles.inputIcon}
          />
          <TextInput
            ref={usernameInputRef}
            style={[styles.input, { height: inputHeight }]}
            placeholder='Username'
            placeholderTextColor={Colors.info}
            value={username}
            onChangeText={setUsername}
            onFocus={() => setFocusedElement('username')}
            autoCapitalize='none'
            autoCorrect={false}
            selectionColor={Colors.secondary}
            cursorColor={Colors.secondary}
          />
        </View>

        <View style={[styles.inputContainer, { marginBottom: spacing }]}>
          <Ionicons
            name='mail-outline'
            size={24}
            color={Colors.info}
            style={styles.inputIcon}
          />
          <TextInput
            ref={emailInputRef}
            style={[styles.input, { height: inputHeight }]}
            placeholder='Email'
            placeholderTextColor={Colors.info}
            value={email}
            onChangeText={setEmail}
            onFocus={() => setFocusedElement('email')}
            autoCapitalize='none'
            autoCorrect={false}
            keyboardType='email-address'
            selectionColor={Colors.secondary}
            cursorColor={Colors.secondary}
          />
        </View>

        <View style={[styles.inputContainer, { marginBottom: spacing * 2 }]}>
          <Ionicons
            name='lock-closed-outline'
            size={24}
            color={Colors.info}
            style={styles.inputIcon}
          />
          <TextInput
            ref={passwordInputRef}
            style={[styles.input, { height: inputHeight }]}
            placeholder='Password'
            placeholderTextColor={Colors.info}
            value={password}
            onChangeText={setPassword}
            onFocus={() => setFocusedElement('password')}
            secureTextEntry={!showPassword}
            autoCapitalize='none'
            autoCorrect={false}
            selectionColor={Colors.secondary}
            cursorColor={Colors.secondary}
          />
          <Pressable
            onPress={() => setShowPassword(!showPassword)}
            style={styles.passwordToggle}
          >
            <Ionicons
              name={showPassword ? 'eye-off-outline' : 'eye-outline'}
              size={24}
              color={Colors.info}
            />
          </Pressable>
        </View>

        {/* Error Message */}
        {error && (
          <ThemedText
            variant='light'
            size='medium'
            style={[styles.errorText, { marginBottom: spacing }]}
          >
            {error}
          </ThemedText>
        )}

        {/* Sign In Button */}
        <Pressable
          ref={signInButtonRef}
          style={({ focused }) => [
            styles.loginButton,
            focused && styles.loginButtonFocused,
            { height: inputHeight }
          ]}
          onPress={handleLogin}
          onFocus={() => setFocusedElement('signIn')}
          disabled={loading}
        >
          <ThemedText variant='dark' size='large' weight='bold'>
            {loading ? 'Signing In...' : 'Sign In'}
          </ThemedText>
        </Pressable>
      </View>
    </ThemedView>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    justifyContent: 'center',
    alignItems: 'center',
  },
  formContainer: {
    width: '35%',
    minWidth: 400,
    alignItems: 'center',
  },
  logo: {
    width: 150,
    height: 150,
  },
  inputContainer: {
    width: '100%',
    position: 'relative',
  },
  input: {
    width: '100%',
    backgroundColor: Colors.primary,
    borderRadius: 8,
    paddingHorizontal: 50,
    fontSize: 18,
    color: Colors.light,
  },
  inputIcon: {
    position: 'absolute',
    left: 16,
    top: '50%',
    transform: [{ translateY: -12 }],
  },
  passwordToggle: {
    position: 'absolute',
    right: 16,
    top: '50%',
    transform: [{ translateY: -12 }],
  },
  loginButton: {
    width: '100%',
    backgroundColor: Colors.secondary,
    justifyContent: 'center',
    alignItems: 'center',
    borderRadius: 8,
    transform: [{ scale: 1 }],
  },
  loginButtonFocused: {
    transform: [{ scale: 1.02 }],
  },
  errorText: {
    color: Colors.danger,
    textAlign: 'center',
  },
});
