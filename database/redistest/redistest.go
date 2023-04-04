// Package redistest implements dockertest with redis for testing.
//
// Use case in TestMain
//
//	var dockertest *redistest.DockertestWrapper
//
//	func TestMain(m *testing.M) {
//		dockertest = redistest.New()
//
//		code := m.Run()
//
//		dockertest.Purge()
//
//		os.Exit(code)
//	}
//
//	func TestFunc(t *testing.T) {
//		t.Parallel()
//
//		repo := NewRepo(dockertest.RedisAddr)
//
//		tests := []struct{
//			name string
//		}{}
//		for _, tt := range tests {
//			tt := tt
//			t.Run(tt.name, func(t *testing.T) {
//				t.Parallel()
//				...
//			})
//		}
//	}
//
//	func TestRun(t *testing.T) {
//		t.Parallel()
//
//		tests := []struct{
//			name string
//		}{}
//		for _, tt := range tests {
//			tt := tt
//			t.Run(tt.name, func(t *testing.T) {
//				t.Parallel()
//				repo := NewRepo(dockertest.RedisAddr)
//				...
//			})
//		}
//	}
package redistest
