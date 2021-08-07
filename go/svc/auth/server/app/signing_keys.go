package app

type SigningKeys struct {
	private interface{}
	public  interface{}
}

func NewSK(private, public interface{}) *SigningKeys {
	return &SigningKeys{
		private: private,
		public:  public,
	}
}

func (sk SigningKeys) Public() interface{} {
	return sk.public
}

func (sk SigningKeys) Private() interface{} {
	return sk.private
}
